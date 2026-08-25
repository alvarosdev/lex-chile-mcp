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
	toolSearchCgrDictamenes    = "search_cgr_dictamenes"
	toolGetCgrDictamen         = "get_cgr_dictamen"
	toolCountCgrJurisprudencia = "count_cgr_jurisprudencia"
)

func ToolNames() []string {
	return []string{toolSearchCgrDictamenes, toolGetCgrDictamen, toolCountCgrJurisprudencia}
}

//go:embed prompts.yaml
var rawPrompts []byte

var expectedPromptNames = []string{
	"search_jurisprudence",
	"analyze_dictamen",
	"explain_dictamen_simply",
	"interpret_dictamen",
}

var allowedPlaceholders = map[string]bool{
	"query":                         true,
	"dictamen_id":                   true,
	"audience":                      true,
	"order":                         true,
	"exact_search":                  true,
	"lang":                          true,
	"tool_search_cgr_dictamenes":    true,
	"tool_get_cgr_dictamen":         true,
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
		"tool_search_cgr_dictamenes":    toolSearchCgrDictamenes,
		"tool_get_cgr_dictamen":         toolGetCgrDictamen,
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
		Description: "Find Contraloría dictámenes: explore counts, search paginated, and read the full document with citation.",
		Arguments: []*mcp.PromptArgument{
			{Name: "query", Title: "Query", Description: "Search text, e.g. quillota or bono", Required: true},
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
}
