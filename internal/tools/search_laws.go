package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// SearchLawsArgs carries the arguments of the search_laws tool. Page and
// PageSize are optional (omitempty keeps them out of the required schema).
type SearchLawsArgs struct {
	Query    string `json:"query" jsonschema:"search text, e.g. \"Ley 21.600\""`
	Page     int    `json:"page,omitempty" jsonschema:"result page number, starting at 1 (default 1)"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"results per page (default 10, max 50)"`
}

// SearchResultOut is one result inside the structured output. The summary
// carries the SAME truncation as the text view — both views describe the
// same scope, no duplicated-complete-summary drift.
type SearchResultOut struct {
	NormID    int64  `json:"norm_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Published string `json:"published"`
	Organism  string `json:"organism"`
	Summary   string `json:"summary"`
}

// SearchLawsOutput is the structured content of search_laws. The text view
// derives from the same data — no drift.
type SearchLawsOutput struct {
	Query      string            `json:"query"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalItems int               `json:"total_items"`
	TotalPages int               `json:"total_pages"`
	Results    []SearchResultOut `json:"results"`
}

// RegisterSearchLaws registers the search_laws tool on the MCP server.
func RegisterSearchLaws(srv *mcp.Server, client bcn.LawClient) {
	registerTool(srv, &mcp.Tool{
		Name: "search_laws",
		Description: "Search Chilean laws, decrees and resolutions in LeyChile (Biblioteca " +
			"del Congreso Nacional de Chile). Returns paginated results with the norm_id " +
			"needed to fetch the full content with get_law. Use total_results and page to " +
			"navigate results.",
	}, makeSearchLaws(client))
}

func makeSearchLaws(client bcn.LawClient) mcp.ToolHandlerFor[SearchLawsArgs, SearchLawsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchLawsArgs) (*mcp.CallToolResult, SearchLawsOutput, error) {
		if strings.TrimSpace(args.Query) == "" {
			return errorResult("query is required"), SearchLawsOutput{}, nil
		}
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 50 {
			args.PageSize = 10
		}

		result, err := client.Search(ctx, bcn.SearchParams{
			Query:    args.Query,
			Page:     args.Page,
			PageSize: args.PageSize,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("search failed: %v", err)), SearchLawsOutput{}, nil
		}

		output := buildSearchOutput(result, args)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatSearchResults(result, args)},
			},
		}, output, nil
	}
}

// buildSearchOutput projects the search response into the structured output.
func buildSearchOutput(result bcn.SearchResponse, args SearchLawsArgs) SearchLawsOutput {
	total := int(result.Pagination.TotalItems)
	totalPages := (total + args.PageSize - 1) / args.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	out := SearchLawsOutput{
		Query:      result.Pagination.Query,
		Page:       args.Page,
		PageSize:   args.PageSize,
		TotalItems: total,
		TotalPages: totalPages,
		Results:    make([]SearchResultOut, 0, len(result.Results)),
	}
	for _, norma := range result.Results {
		out.Results = append(out.Results, SearchResultOut{
			NormID:    norma.IDNorma,
			Type:      norma.Tipo,
			Title:     norma.TituloNorma,
			Published: norma.FechaPublicacion,
			Organism:  norma.Organismo,
			Summary:   truncate(norma.Resumen, 600),
		})
	}
	return out
}

// formatSearchResults renders the search response for the LLM: header with
// pagination, then one entry per norm with the fields the model needs to
// decide (type, number, title, dates, organism, summary) and the norm_id
// to fetch the full content. The summary is truncated identically in the
// text and structured views.
func formatSearchResults(result bcn.SearchResponse, args SearchLawsArgs) string {
	total := int(result.Pagination.TotalItems)
	totalPages := (total + args.PageSize - 1) / args.PageSize
	if totalPages < 1 {
		totalPages = 1
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for %q — page %d of %d (%d total results)\n\n",
		result.Pagination.Query, args.Page, totalPages, total)

	for i, norma := range result.Results {
		fmt.Fprintf(&b, "%d. %s | %s | norm_id: %d\n", i+1, norma.Norma, norma.Tipo, norma.IDNorma)
		fmt.Fprintf(&b, "   %s\n", norma.TituloNorma)
		if norma.FechaPublicacion != "" {
			fmt.Fprintf(&b, "   Published: %s · Organism: %s\n", norma.FechaPublicacion, norma.Organismo)
		}
		if norma.Resumen != "" {
			fmt.Fprintf(&b, "   Summary: %s\n", truncate(norma.Resumen, 600))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// truncate cuts s to maxLen runes, adding an ellipsis when truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
