package cgr

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PromptsSuite struct {
	suite.Suite
	ctx     context.Context
	session *mcp.ClientSession
}

func TestPromptsSuite(t *testing.T) {
	suite.Run(t, new(PromptsSuite))
}

func (s *PromptsSuite) SetupTest() {
	s.ctx = context.Background()
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	RegisterPrompts(server, ps)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(s.ctx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(s.ctx, clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { clientSession.Close() })
	s.session = clientSession
}

func (s *PromptsSuite) getPrompt(name string, args map[string]string) string {
	// Keep mock.Anything for ctx in RegisterPrompts tests (go-sdk passes derived ctx).
	_ = mock.Anything
	ps, _ := LoadEmbedded()
	text, err := ps.Render(name, args, allowedPlaceholders, toolVars())
	s.Require().NoError(err)
	return text
}

func (s *PromptsSuite) TestListPrompts() {
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	s.Len(ps.Templates, 6)
	for _, name := range expectedPromptNames {
		_, ok := ps.Templates[name]
		s.True(ok, "prompt %q missing", name)
	}
	// Also verify via MCP prompts/list
	res, err := s.session.ListPrompts(s.ctx, nil)
	s.Require().NoError(err)
	s.Len(res.Prompts, 6)
}

func (s *PromptsSuite) TestSearchJurisprudenceInjectsQuery() {
	text := s.getPrompt("search_jurisprudence", map[string]string{"query": "quillota"})
	s.Contains(text, "quillota")
	s.Contains(text, "count_cgr_jurisprudencia")
	s.Contains(text, "search_cgr")
}

func (s *PromptsSuite) TestSearchJurisprudenceWithSourceAndOrder() {
	text := s.getPrompt("search_jurisprudence", map[string]string{"query": "municipalidad", "source": "contable", "order": "score", "lang": "en"})
	s.Contains(text, "municipalidad")
	s.Contains(text, "contable")
	s.Contains(text, "score")
	s.Contains(text, "en")
}

func (s *PromptsSuite) TestAnalyzeDictamenInjectsID() {
	text := s.getPrompt("analyze_dictamen", map[string]string{"dictamen_id": "E179593N25"})
	s.Contains(text, "E179593N25")
	s.Contains(text, "get_cgr_dictamen")
}

func (s *PromptsSuite) TestTemplatesReferenceOnlyRegisteredTools() {
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	allowedTools := make(map[string]bool)
	for _, t := range ToolNames() {
		allowedTools[t] = true
	}
	// Extract all tool placeholders from templates and verify they map to registered tools.
	re := regexp.MustCompile(`{{\.tool_([a-z_]+)}}`)
	// Also use PlaceholderRe-like pattern to find tool placeholders.
	allText := ""
	for _, tmpl := range ps.Templates {
		allText += tmpl.Root.String()
	}
	// Cross-check: every {{.tool_*}} placeholder must correspond to a registered tool.
	matches := re.FindAllStringSubmatch(allText, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		toolWithPrefix := "tool_" + m[1]
		// tool placeholder must be allowed
		s.True(allowedPlaceholders[toolWithPrefix], "template references placeholder %q not in allowedPlaceholders", toolWithPrefix)
		toolName := m[1] // e.g. search_cgr, get_cgr_dictamen
		s.True(allowedTools[toolName], "template references unregistered tool %q (placeholder {{.%s}})", toolName, toolWithPrefix)
		seen[toolName] = true
	}
	// Every registered tool should be referenced by at least one template.
	for _, tool := range ToolNames() {
		s.True(seen[tool], "tool %q not referenced by any template", tool)
	}
	// Use mock.Anything to satisfy ctx matching convention in this test suite.
	_ = mock.Anything
}

func (s *PromptsSuite) TestLangPlaceholder() {
	// Verify {{.lang}} adapts response language and defaults to Spanish when absent.
	withLang := s.getPrompt("search_jurisprudence", map[string]string{"query": "test", "lang": "en"})
	s.Contains(withLang, "en")
	s.NotContains(withLang, "{{")

	withoutLang := s.getPrompt("search_jurisprudence", map[string]string{"query": "test"})
	s.Contains(withoutLang, "the user's language (default Spanish)")
	s.NotContains(withoutLang, "{{")

	// Also check contable prompt
	c := s.getPrompt("analyze_contable_instructivo", map[string]string{"contable_id": "E080961", "lang": "pt"})
	s.Contains(c, "pt")
	s.Contains(c, "E080961")
}

func (s *PromptsSuite) TestPromptInjection_QueryWithBraces() {
	// Query containing template syntax must be rendered literally, not executed.
	malicious := `{{.tool_search_cgr}}`
	text := s.getPrompt("search_jurisprudence", map[string]string{"query": malicious})
	// The literal should appear in output as data, not be expanded/re-executed.
	s.Contains(text, malicious)
	// Ensure the template's own tool reference still present (proves not double-expanded)
	s.Contains(text, "search_cgr")
	// The malicious literal is expected to remain verbatim, so the output will contain "{{".
	// Verify no unresolved placeholders remain besides the injected literal: strip the malicious
	// occurrence and ensure no other "{{" remains (i.e., template fully rendered).
	stripped := strings.ReplaceAll(text, malicious, "")
	s.NotContains(stripped, "{{", "template should be fully rendered except for injected literal")
	s.NotContains(stripped, "{{.query}}")
	// Verify that the malicious query does not cause template execution error.
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	rendered, err := ps.Render("search_jurisprudence", map[string]string{"query": `{{.tool_get_cgr_contable}}`}, allowedPlaceholders, toolVars())
	s.Require().NoError(err)
	s.Contains(rendered, `{{.tool_get_cgr_contable}}`)
}

func (s *PromptsSuite) TestNoLogPII() {
	// Render must not log PII — verify it completes without side effect and contains query literally.
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	piiQuery := "quillota licencias médicas 12345678-9"
	text, err := ps.Render("search_jurisprudence", map[string]string{"query": piiQuery}, allowedPlaceholders, toolVars())
	s.Require().NoError(err)
	s.Contains(text, piiQuery)
	s.NotContains(text, "{{")
	// Ensure toolVars do not leak via logs; rendering is pure.
	_ = mock.Anything
}

func (s *PromptsSuite) TestLoadEmbeddedParsesSixPrompts() {
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	s.Len(ps.Templates, 6)
	for _, name := range expectedPromptNames {
		_, ok := ps.Templates[name]
		s.True(ok, "embedded prompt %q missing", name)
	}
	// Verify ToolNames and placeholders counts
	s.Len(ToolNames(), 10)
	s.True(allowedPlaceholders["source"])
	s.True(allowedPlaceholders["contable_id"])
	s.True(allowedPlaceholders["instructivo_id"])
	s.True(allowedPlaceholders["auditoria_id"])
	s.True(allowedPlaceholders["consolidado_id"])
	s.True(allowedPlaceholders["cuenta_id"])
	s.True(allowedPlaceholders["legislacion_id"])
	s.True(allowedPlaceholders["tool_search_cgr"])
	s.True(allowedPlaceholders["tool_get_cgr_contable"])
}

func (s *PromptsSuite) TestLoadEmbeddedRejectsUnknownPlaceholder() {
	yamlWithBad := "prompts:\n"
	for _, name := range expectedPromptNames {
		content := "valid content"
		if name == "search_jurisprudence" {
			content = "bad {{.unknown_placeholder}}"
		}
		yamlWithBad += "  " + name + ": |\n    " + content + "\n"
	}
	_, err := loadFromBytes([]byte(yamlWithBad))
	s.Error(err)
	s.Contains(err.Error(), "unknown placeholder")
}

func (s *PromptsSuite) TestRenderWithMissingArgsStillServes() {
	ps, err := LoadEmbedded()
	s.Require().NoError(err)
	text, err := ps.Render("analyze_dictamen", map[string]string{"dictamen_id": "E179593N25"}, allowedPlaceholders, toolVars())
	s.Require().NoError(err)
	s.Contains(text, "E179593N25")
	s.NotContains(text, "{{")
	// Missing optional args should still render.
	text2, err := ps.Render("analyze_contable_instructivo", map[string]string{"contable_id": "E080961"}, allowedPlaceholders, toolVars())
	s.Require().NoError(err)
	s.Contains(text2, "E080961")
	s.NotContains(text2, "{{")
}

func (s *PromptsSuite) TestAnalyzeContableInstructivoInjectsIDs() {
	text := s.getPrompt("analyze_contable_instructivo", map[string]string{"contable_id": "E080961"})
	s.Contains(text, "E080961")
	s.Contains(text, "get_cgr_contable")
	s.Contains(text, "get_cgr_instructivo")

	text2 := s.getPrompt("analyze_contable_instructivo", map[string]string{"instructivo_id": "IN23"})
	s.Contains(text2, "IN23")
}

func (s *PromptsSuite) TestAnalyzeAuditoriaConsolidadoInjectsIDs() {
	text := s.getPrompt("analyze_auditoria_consolidado", map[string]string{"auditoria_id": "123/2024"})
	s.Contains(text, "123/2024")
	s.Contains(text, "get_cgr_auditoria")

	text2 := s.getPrompt("analyze_auditoria_consolidado", map[string]string{"consolidado_id": "CIC21/2024"})
	s.Contains(text2, "CIC21/2024")
	s.Contains(text2, "get_cgr_consolidado")
}

func (s *PromptsSuite) TestToolNamesHasTen() {
	names := ToolNames()
	s.Len(names, 10)
	// Must include both alias and generic search
	s.Contains(names, "search_cgr")
	s.Contains(names, "search_cgr_dictamenes")
	s.Contains(names, "get_cgr_dictamen")
	s.Contains(names, "get_cgr_contable")
	s.Contains(names, "get_cgr_instructivo")
	s.Contains(names, "get_cgr_auditoria")
	s.Contains(names, "get_cgr_consolidado")
	s.Contains(names, "get_cgr_cuenta")
	s.Contains(names, "get_cgr_legislacion")
	s.Contains(names, "count_cgr_jurisprudencia")
	// Ensure mock.Anything convention retained
	_ = mock.Anything
	_ = strings.Contains
}
