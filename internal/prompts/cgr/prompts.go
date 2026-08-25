// Package cgr provides the curated MCP prompts for Contraloría (CGR) domain.
package cgr

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	promptinternal "github.com/alvarosdev/lex-chile-mcp/internal/prompts/internal"
)

const (
	toolSearchCgr              = "search_cgr"
	toolSearchCgrDictamenes    = "search_cgr_dictamenes"
	toolGetCgrDictamen         = "get_cgr_dictamen"
	toolGetCgrInstructivo      = "get_cgr_instructivo"
	toolGetCgrContable         = "get_cgr_contable"
	toolGetCgrAuditoria        = "get_cgr_auditoria"
	toolGetCgrConsolidado      = "get_cgr_consolidado"
	toolGetCgrCuenta           = "get_cgr_cuenta"
	toolGetCgrLegislacion      = "get_cgr_legislacion"
	toolCountCgrJurisprudencia = "count_cgr_jurisprudencia"

	// Aliases for assignment naming (without Cgr prefix) — kept for compat.
	toolGetInstructivo = toolGetCgrInstructivo
	toolGetContable    = toolGetCgrContable
	toolGetAuditoria   = toolGetCgrAuditoria
	toolGetConsolidado = toolGetCgrConsolidado
	toolGetCuenta      = toolGetCgrCuenta
	toolGetLegislacion = toolGetCgrLegislacion
)

func ToolNames() []string {
	return []string{
		toolSearchCgr,
		toolSearchCgrDictamenes,
		toolGetCgrDictamen,
		toolGetCgrInstructivo,
		toolGetCgrContable,
		toolGetCgrAuditoria,
		toolGetCgrConsolidado,
		toolGetCgrCuenta,
		toolGetCgrLegislacion,
		toolCountCgrJurisprudencia,
	}
}

//go:embed prompts.yaml
var rawPrompts []byte

var expectedPromptNames = []string{
	"search_jurisprudence",
	"analyze_dictamen",
	"explain_dictamen_simply",
	"interpret_dictamen",
	"analyze_contable_instructivo",
	"analyze_auditoria_consolidado",
}

var allowedPlaceholders = map[string]bool{
	"query":                         true,
	"dictamen_id":                   true,
	"audience":                      true,
	"order":                         true,
	"exact_search":                  true,
	"lang":                          true,
	"source":                        true,
	"contable_id":                   true,
	"instructivo_id":                true,
	"auditoria_id":                  true,
	"consolidado_id":                true,
	"cuenta_id":                     true,
	"legislacion_id":                true,
	"tool_search_cgr":               true,
	"tool_search_cgr_dictamenes":    true,
	"tool_get_cgr_dictamen":         true,
	"tool_get_cgr_instructivo":      true,
	"tool_get_cgr_contable":         true,
	"tool_get_cgr_auditoria":        true,
	"tool_get_cgr_consolidado":      true,
	"tool_get_cgr_cuenta":           true,
	"tool_get_cgr_legislacion":      true,
	"tool_count_cgr_jurisprudencia": true,
}

type PromptSet = promptinternal.PromptSet

func LoadEmbedded() (*PromptSet, error) {
	return promptinternal.Load(rawPrompts, expectedPromptNames, allowedPlaceholders)
}

func loadFromBytes(data []byte) (*PromptSet, error) {
	return promptinternal.Load(data, expectedPromptNames, allowedPlaceholders)
}

func toolVars() map[string]string {
	return map[string]string{
		"tool_search_cgr":               toolSearchCgr,
		"tool_search_cgr_dictamenes":    toolSearchCgrDictamenes,
		"tool_get_cgr_dictamen":         toolGetCgrDictamen,
		"tool_get_cgr_instructivo":      toolGetCgrInstructivo,
		"tool_get_cgr_contable":         toolGetCgrContable,
		"tool_get_cgr_auditoria":        toolGetCgrAuditoria,
		"tool_get_cgr_consolidado":      toolGetCgrConsolidado,
		"tool_get_cgr_cuenta":           toolGetCgrCuenta,
		"tool_get_cgr_legislacion":      toolGetCgrLegislacion,
		"tool_count_cgr_jurisprudencia": toolCountCgrJurisprudencia,
	}
}

func RegisterPrompts(srv *mcp.Server, ps *PromptSet) {
	add := func(p *mcp.Prompt, name string) {
		srv.AddPrompt(p, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			text, err := ps.Render(name, req.Params.Arguments, allowedPlaceholders, toolVars())
			if err != nil {
				text = fmt.Sprintf("prompt render error for %q: %v", name, err)
			}
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{{
					Role:    mcp.Role("user"),
					Content: &mcp.TextContent{Text: text},
				}},
			}, nil
		})
	}

	add(&mcp.Prompt{
		Name:        "search_jurisprudence",
		Title:       "Find Contraloría jurisprudence",
		Description: "Find Contraloría jurisprudence across all sources: explore counts, search paginated, and read the full document with citation.",
		Arguments: []*mcp.PromptArgument{
			{Name: "query", Title: "Query", Description: "Search text, e.g. quillota or bono", Required: true},
			{Name: "source", Title: "Source", Description: "Source: dictamenes, instructivos, contable, auditoria, consolidados, cuentas, legislacion, web, todos (default dictamenes)", Required: false},
			{Name: "order", Title: "Order", Description: "Order: date (newest), dateasc (oldest), score (relevance)", Required: false},
			{Name: "exact_search", Title: "Exact search", Description: "Exact match (true/false)", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "search_jurisprudence")

	add(&mcp.Prompt{
		Name:        "analyze_dictamen",
		Title:       "Analyze a Contraloría dictamen",
		Description: "Analyze a dictamen: materia, descriptores, criterio, fuentes legales and document, with citation and hedge.",
		Arguments: []*mcp.PromptArgument{
			{Name: "dictamen_id", Title: "Dictamen id", Description: "The dictamen id (dictamen_id) from search_cgr_dictamenes, e.g. E179593N25", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "analyze_dictamen")

	add(&mcp.Prompt{
		Name:        "explain_dictamen_simply",
		Title:       "Explain a dictamen in plain language",
		Description: "Explain a dictamen without legal jargon, citing url/pdf_url, with no-legal-advice disclaimer.",
		Arguments: []*mcp.PromptArgument{
			{Name: "dictamen_id", Title: "Dictamen id", Description: "The dictamen id (dictamen_id) from search_cgr_dictamenes", Required: true},
			{Name: "audience", Title: "Audience", Description: "Optional target audience", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "explain_dictamen_simply")

	add(&mcp.Prompt{
		Name:        "interpret_dictamen",
		Title:       "Interpret a Contraloría dictamen without bias",
		Description: "Interpret a dictamen with the structured 4-step method, hierarchy and anti-bias, citing the source.",
		Arguments: []*mcp.PromptArgument{
			{Name: "dictamen_id", Title: "Dictamen id", Description: "The dictamen id (dictamen_id) from search_cgr_dictamenes", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "interpret_dictamen")

	add(&mcp.Prompt{
		Name:        "analyze_contable_instructivo",
		Title:       "Analyze contable dictamen or instructivo",
		Description: "Analyze a contable dictamen (OFE/E) or instructivo (IN): normativa contable, parte/materia and texto, citing url/pdf_url.",
		Arguments: []*mcp.PromptArgument{
			{Name: "contable_id", Title: "Contable id", Description: "Contable id (e.g. E080961 or OFE...), optional if instructivo_id provided", Required: false},
			{Name: "instructivo_id", Title: "Instructivo id", Description: "Instructivo id (e.g. IN23...), optional if contable_id provided", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "analyze_contable_instructivo")

	add(&mcp.Prompt{
		Name:        "analyze_auditoria_consolidado",
		Title:       "Analyze auditoría or consolidado",
		Description: "Analyze an auditoría Informe Final or consolidado CIC: resena/contenido_extraido (truncated), pdf link and fiscalizacion method.",
		Arguments: []*mcp.PromptArgument{
			{Name: "auditoria_id", Title: "Auditoría id", Description: "Auditoría id (e.g. 123/2024 or E...N...), optional if consolidado_id provided", Required: false},
			{Name: "consolidado_id", Title: "Consolidado id", Description: "Consolidado id (e.g. CIC21/2024), optional if auditoria_id provided", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "analyze_auditoria_consolidado")

	// Ensure alias vars are referenced so they are not flagged as unused.
	_ = toolGetContable
	_ = toolGetInstructivo
	_ = toolGetAuditoria
	_ = toolGetConsolidado
	_ = toolGetCuenta
	_ = toolGetLegislacion
}
