package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// SearchCgrDictamenesArgs carries the arguments of the search_cgr_dictamenes tool.
type SearchCgrDictamenesArgs struct {
	Query       string `json:"query" jsonschema:"search text, e.g. \"quillota\" or \"bono\" (empty lists recent)"`
	ExactSearch bool   `json:"exact_search,omitempty" jsonschema:"exact match (default false)"`
	Order       string `json:"order,omitempty" jsonschema:"result order: date (newest first, default), dateasc (oldest first), score (relevance)"`
	Page        int    `json:"page,omitempty" jsonschema:"result page number, starting at 1 (default 1, 20 per page)"`
}

// RegisterSearchCgrDictamenes registers the search_cgr_dictamenes tool (alias to search_cgr with source dictamenes).
func RegisterSearchCgrDictamenes(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_cgr_dictamenes",
		Description: "Search Chilean Contraloría dictámenes by text. Returns a paginated list (20 per page) with dictamen_id, n_dictamen, fecha_documento, materia, descriptores, criterio, origen, caracter and the HTML/PDF URLs for citation. Use get_cgr_dictamen(dictamen_id) to fetch the full document. Supports order date (newest), dateasc (oldest), score (relevance) and exact_search.",
	}, makeSearchCgrDictamenes(client))
}

func makeSearchCgrDictamenes(client cgr.CgrClient) mcp.ToolHandlerFor[SearchCgrDictamenesArgs, SearchCgrDictamenesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchCgrDictamenesArgs) (*mcp.CallToolResult, SearchCgrDictamenesOutput, error) {
		order := args.Order
		if order == "" {
			order = "date"
		}
		if order != "date" && order != "dateasc" && order != "score" {
			return errorResult(fmt.Sprintf("order must be date, dateasc or score (got %q)", args.Order)), SearchCgrDictamenesOutput{}, nil
		}
		page := args.Page
		if page == 0 {
			page = 1
		}
		if page < 1 {
			return errorResult("page must be >= 1"), SearchCgrDictamenesOutput{}, nil
		}
		if page > 500 {
			return errorResult("page must be <= 500"), SearchCgrDictamenesOutput{}, nil
		}
		query := args.Query
		if runes := []rune(query); len(runes) > 500 {
			query = string(runes[:500])
		}
		// Trim space for safety, mimic search_cgr behavior
		query = strings.TrimSpace(query)
		// Note: query can be empty -> lists recent, so don't error on empty after trim if original empty
		// If original was only spaces, trimmed becomes empty which is allowed
		params := cgr.SearchParams{
			Query:       query,
			ExactSearch: args.ExactSearch,
			Order:       order,
			Page:        page,
			Source:      "dictamenes",
		}
		result, err := client.Search(ctx, params)
		if err != nil {
			return errorResult(fmt.Sprintf("search cgr dictamenes failed: %v", err)), SearchCgrDictamenesOutput{}, nil
		}
		output := buildSearchCgrOutput(result, "dictamenes")
		// Convert args to generic for formatting
		genericArgs := SearchCgrArgs{
			Query:       args.Query,
			ExactSearch: args.ExactSearch,
			Order:       args.Order,
			Page:        args.Page,
			Source:      "dictamenes",
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatSearchCgrResults(result, genericArgs)},
			},
		}, output, nil
	}
}

// buildCgrSearchOutput is kept for backward compat; delegates to generic builder.
func buildCgrSearchOutput(result cgr.SearchResponse) SearchCgrDictamenesOutput {
	return buildSearchCgrOutput(result, "dictamenes")
}

// formatCgrSearchResults is kept for backward compat; delegates to generic formatter.
func formatCgrSearchResults(result cgr.SearchResponse, args SearchCgrDictamenesArgs) string {
	genericArgs := SearchCgrArgs{
		Query:       args.Query,
		ExactSearch: args.ExactSearch,
		Order:       args.Order,
		Page:        args.Page,
		Source:      "dictamenes",
	}
	return formatSearchCgrResults(result, genericArgs)
}
