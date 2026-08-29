package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// maxResults caps the matches a find_in_norm response lists; the total is
// always signaled (output-budget: no silent truncation).
const maxResults = 20

// FindInNormArgs carries the arguments of the find_in_norm tool. Query is
// plain literal text (case-insensitive substring, never a regex).
type FindInNormArgs struct {
	NormID      int64  `json:"norm_id" jsonschema:"the norm id (norm_id) from search_laws results"`
	Query       string `json:"query" jsonschema:"plain text to find inside the norm (literal case-insensitive substring, never a regex)"`
	VersionDate string `json:"version_date,omitempty" jsonschema:"version in force at this date (YYYY-MM-DD, optional — defaults to the latest version)"`
}

// SectionMatchOut is one match inside the structured output: the section
// identifier to fetch with get_law(section_id=...), its real size and
// article count, and — for content matches — the context snippet.
type SectionMatchOut struct {
	Name         string `json:"name"`
	SectionID    int64  `json:"section_id"`
	CharCount    int    `json:"char_count"`
	ArticleCount int    `json:"article_count"`
	Snippet      string `json:"snippet,omitempty"`
}

// FindInNormOutput is the structured content of find_in_norm. Results are
// ranked (name matches first, then content, document order within each
// group) and capped at maxResults; MatchedTotal carries the real count.
type FindInNormOutput struct {
	Results      []SectionMatchOut `json:"results"`
	MatchedTotal int               `json:"matched_total"`
}

// RegisterFindInNorm registers the find_in_norm tool on the MCP server.
func RegisterFindInNorm(srv *mcp.Server, client bcn.LawClient) {
	registerTool(srv, &mcp.Tool{
		Name: "find_in_norm",
		Description: "Find sections inside ONE Chilean norm by plain text (norm_id from search_laws). " +
			"Case-insensitive literal substring over section names and content; name matches rank " +
			"first. Returns the section_id of every match with its size and a context snippet — " +
			"fetch the content with get_law(section_id=...). Drill pattern: search_laws → " +
			"find_in_norm → get_law(section_id). Faster than walking the summary TOC when you " +
			"already know what you are looking for (an article number, a phrase).",
	}, makeFindInNorm(client))
}

func makeFindInNorm(client bcn.LawClient) mcp.ToolHandlerFor[FindInNormArgs, FindInNormOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args FindInNormArgs) (*mcp.CallToolResult, FindInNormOutput, error) {
		if args.NormID <= 0 {
			return errorResult("norm_id must be a positive number"), FindInNormOutput{}, nil
		}
		if strings.TrimSpace(args.Query) == "" {
			return errorResult("query is required"), FindInNormOutput{}, nil
		}
		if err := validateVersionDate(args.VersionDate); err != nil {
			return errorResult(err.Error()), FindInNormOutput{}, nil
		}

		result, err := client.SearchNorma(ctx, bcn.NormaQuery{NormID: args.NormID, VersionDate: args.VersionDate}, args.Query)
		if err != nil {
			if errors.Is(err, bcn.ErrNormaNotFound) {
				return errorResult(fmt.Sprintf("norma not found: norm_id %d does not exist in LeyChile", args.NormID)), FindInNormOutput{}, nil
			}
			return errorResult(fmt.Sprintf("find in norm failed: %v", err)), FindInNormOutput{}, nil
		}

		output := buildFindInNormOutput(result)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatFindInNorm(result, args)},
			},
		}, output, nil
	}
}

// buildFindInNormOutput projects the search result into the structured
// output: matches capped at maxResults, MatchedTotal always the real
// count (the cap is signaled, never silent).
func buildFindInNormOutput(result bcn.NormaSearchResult) FindInNormOutput {
	out := FindInNormOutput{
		Results:      make([]SectionMatchOut, 0, min(len(result.Matches), maxResults)),
		MatchedTotal: len(result.Matches),
	}
	for _, m := range result.Matches {
		if len(out.Results) == maxResults {
			break
		}
		out.Results = append(out.Results, SectionMatchOut{
			Name:         m.Name,
			SectionID:    m.SectionID,
			CharCount:    m.CharCount,
			ArticleCount: m.ArticleCount,
			Snippet:      m.Snippet,
		})
	}
	return out
}

// formatFindInNorm renders the search for the LLM: header with the query
// and the walked scope, one line per match (identifier + size, snippet
// indented for content matches), and the cap signal when matches were
// truncated — never a silent cut.
func formatFindInNorm(result bcn.NormaSearchResult, args FindInNormArgs) string {
	var b strings.Builder
	if len(result.Matches) == 0 {
		fmt.Fprintf(&b, "No matches for %q — walked %d sections of norm_id %d. "+
			"Try a shorter or different phrase; get_law_summary shows the structure with section ids.\n",
			args.Query, result.Walked, args.NormID)
		return b.String()
	}

	fmt.Fprintf(&b, "Matches for %q in norm_id %d (%d sections walked)\n\n",
		args.Query, args.NormID, result.Walked)
	listed := 0
	for _, m := range result.Matches {
		if listed == maxResults {
			break
		}
		listed++
		fmt.Fprintf(&b, "- %s · section_id: %d · %s chars · %s\n",
			m.Name, m.SectionID, humanCount(m.CharCount), formatArticles(m.ArticleCount))
		if m.Snippet != "" {
			fmt.Fprintf(&b, "  > %s\n", collapseSnippetLine(m.Snippet))
		}
	}

	if len(result.Matches) > maxResults {
		fmt.Fprintf(&b, "\nMatched %d sections — showing first %d; narrow the query.\n",
			len(result.Matches), maxResults)
	}
	return b.String()
}

// collapseSnippetLine flattens a snippet to a single display line.
func collapseSnippetLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
