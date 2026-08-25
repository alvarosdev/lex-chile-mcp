// Package bcn provides the curated MCP prompts for LeyChile (BCN) domain.
// Each prompt is a PURE template: it injects the received arguments into a
// message that guides the model on how to use the tools correctly. Serving a
// prompt never touches the BCN API.
//
// Templates are baked into the binary via go:embed (prompts.yaml) and
// rendered with text/template + named placeholders.
package bcn

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	promptinternal "github.com/alvarosdev/lex-chile-mcp/internal/prompts/internal"
)

// Tool names referenced by the prompt templates.
const (
	toolSearchLaws    = "search_laws"
	toolGetLaw        = "get_law"
	toolGetLawSummary = "get_law_summary"
	toolGetLawHistory = "get_law_history"
)

// ToolNames returns the tool names the prompts reference.
func ToolNames() []string {
	return []string{toolSearchLaws, toolGetLaw, toolGetLawSummary, toolGetLawHistory}
}

//go:embed prompts.yaml
var rawPrompts []byte

// expectedPromptNames is the exact set of prompts the server must expose.
var expectedPromptNames = []string{
	"analyze_law",
	"search_legal_topic",
	"compare_law_versions",
	"trace_law_history",
	"check_law_validity",
	"explain_law_simply",
	"law_research_workflow",
	"answer_constitutional_question",
	"check_norm_constitutionality",
	"interpret_law",
}

// allowedPlaceholders is the closed whitelist for template vars.
var allowedPlaceholders = map[string]bool{
	"norm_id":              true,
	"topic":                true,
	"aspect":               true,
	"from_date":            true,
	"to_date":              true,
	"date":                 true,
	"audience":             true,
	"question":             true,
	"article_hint":         true,
	"version_date":         true,
	"lang":                 true,
	"tool_search_laws":     true,
	"tool_get_law":         true,
	"tool_get_law_summary": true,
	"tool_get_law_history": true,
}

// PromptSet holds the parsed, validated prompt templates.
type PromptSet = promptinternal.PromptSet

// LoadEmbedded reads and validates the embedded prompts.yaml contract.
func LoadEmbedded() (*PromptSet, error) {
	return promptinternal.Load(rawPrompts, expectedPromptNames, allowedPlaceholders)
}

// loadFromBytes is exposed for tests — validates raw YAML against the same
// expected names and allowed placeholders.
func loadFromBytes(data []byte) (*PromptSet, error) {
	return promptinternal.Load(data, expectedPromptNames, allowedPlaceholders)
}

func toolVars() map[string]string {
	return map[string]string{
		"tool_search_laws":     toolSearchLaws,
		"tool_get_law":         toolGetLaw,
		"tool_get_law_summary": toolGetLawSummary,
		"tool_get_law_history": toolGetLawHistory,
	}
}

// RegisterPrompts registers the curated prompts on the MCP server.
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
		Name:        "analyze_law",
		Title:       "Analyze a Chilean law",
		Description: "Structured legal analysis of a norm: purpose, scope, obligations, sanctions and entry into force, with citations to the actual text.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "aspect", Title: "Aspect", Description: "Optional analysis dimension to focus on (purpose, obligations, sanctions...)", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "analyze_law")

	add(&mcp.Prompt{
		Name:        "search_legal_topic",
		Title:       "Find norms about a topic",
		Description: "Guided search over LeyChile: pick a good query, read summaries before opening norms, and verify with the full text.",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Title: "Topic", Description: "The legal topic to search for", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "search_legal_topic")

	add(&mcp.Prompt{
		Name:        "compare_law_versions",
		Title:       "Compare two versions of a law",
		Description: "Compare a norm between two dates using historical versions (version_date), reporting what changed and when.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "from_date", Title: "From date", Description: "Start date (YYYY-MM-DD)", Required: true},
			{Name: "to_date", Title: "To date", Description: "End date (YYYY-MM-DD)", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "compare_law_versions")

	add(&mcp.Prompt{
		Name:        "trace_law_history",
		Title:       "Trace what modified a law",
		Description: "Trace the legislative history of a norm: identify the laws that modified it and present a chronological timeline.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "trace_law_history")

	add(&mcp.Prompt{
		Name:        "check_law_validity",
		Title:       "Is this norm in force?",
		Description: "Check whether a norm is in force, derogated, or in force at a given date, based on its metadata and validity window.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "date", Title: "Date", Description: "Optional date to check validity at (YYYY-MM-DD)", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "check_law_validity")

	add(&mcp.Prompt{
		Name:        "explain_law_simply",
		Title:       "Explain a law in plain language",
		Description: "Explain a norm without legal jargon, citing the source article for every claim, with a no-legal-advice disclaimer.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "audience", Title: "Audience", Description: "Optional target audience (e.g. students, small business owners)", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "explain_law_simply")

	add(&mcp.Prompt{
		Name:        "law_research_workflow",
		Title:       "Research a law section by section",
		Description: "Efficient reading of a norm: summary and table of contents first, then only the relevant sections — avoiding the full text of long laws.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "question", Title: "Question", Description: "Optional research question to focus the reading", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "law_research_workflow")

	add(&mcp.Prompt{
		Name:        "answer_constitutional_question",
		Title:       "Answer a constitutional question",
		Description: "Answer a question about the Chilean Constitution (Decreto 100, 242302): locate the relevant chapters/articles via the table of contents and cite the actual text. Supports historical versions via version_date.",
		Arguments: []*mcp.PromptArgument{
			{Name: "question", Title: "Question", Description: "The constitutional question to answer (e.g. '¿qué dice sobre el derecho de propiedad?')", Required: true},
			{Name: "article_hint", Title: "Article hint", Description: "Optional article hint (free text, e.g. '19', '19 Nº24', '93', 'transitoria primera') to prioritize a TOC entry", Required: false},
			{Name: "version_date", Title: "Version date", Description: "Optional version in force at this date (YYYY-MM-DD) — defaults to the latest version", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "answer_constitutional_question")

	add(&mcp.Prompt{
		Name:        "check_norm_constitutionality",
		Title:       "Check norm vs Constitution",
		Description: "Assess whether a norm is compatible with the Chilean Constitution (Decreto 100, 242302) by contrasting the relevant sections side-by-side. Supports historical versions via version_date.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results of the norm to contrast", Required: true},
			{Name: "question", Title: "Question", Description: "Optional focus question (e.g. '¿vulnera igualdad ante la ley?')", Required: false},
			{Name: "version_date", Title: "Version date", Description: "Optional version in force at this date (YYYY-MM-DD) — applied to both norms for temporal coherence", Required: false},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "check_norm_constitutionality")

	add(&mcp.Prompt{
		Name:        "interpret_law",
		Title:       "Interpret a Chilean law without bias",
		Description: "Interpret a Chilean law with a structured method (hierarchy, 5 elements of arts. 19-24 Código Civil, anti-bias), citing LeyChile and distinguishing desirable vs vigente law.",
		Arguments: []*mcp.PromptArgument{
			{Name: "norm_id", Title: "Norm id", Description: "The norm id (norm_id) from search_laws results", Required: true},
			{Name: "lang", Title: "Language", Description: "Response language (e.g. es, en, pt); default Spanish if not specified", Required: false},
		},
	}, "interpret_law")
}
