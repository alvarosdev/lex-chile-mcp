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

// CgrSearchResultOut is one result inside the structured output.
type CgrSearchResultOut struct {
	DictamenID   string `json:"dictamen_id"`
	NDictamen    string `json:"n_dictamen"`
	FechaDoc     string `json:"fecha_documento"`
	Materia      string `json:"materia"`
	Descriptores string `json:"descriptores"`
	Criterio     string `json:"criterio"`
	Origen       string `json:"origen"`
	Caracter     string `json:"caracter"`
	URL          string `json:"url"`
	PDFURL       string `json:"pdf_url"`
}

// CgrSearchPaginationOut is the pagination block for CGR search.
type CgrSearchPaginationOut struct {
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// SearchCgrDictamenesOutput is the structured content of search_cgr_dictamenes.
type SearchCgrDictamenesOutput struct {
	Results    []CgrSearchResultOut   `json:"results"`
	Pagination CgrSearchPaginationOut `json:"pagination"`
}

// RegisterSearchCgrDictamenes registers the search_cgr_dictamenes tool.
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
		params := cgr.SearchParams{
			Query:       args.Query,
			ExactSearch: args.ExactSearch,
			Order:       order,
			Page:        page,
		}
		result, err := client.SearchDictamenes(ctx, params)
		if err != nil {
			return errorResult(fmt.Sprintf("search cgr dictamenes failed: %v", err)), SearchCgrDictamenesOutput{}, nil
		}
		output := buildCgrSearchOutput(result)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatCgrSearchResults(result, args)},
			},
		}, output, nil
	}
}

func buildCgrSearchOutput(result cgr.SearchResponse) SearchCgrDictamenesOutput {
	results := make([]CgrSearchResultOut, 0, len(result.Results))
	for _, r := range result.Results {
		results = append(results, CgrSearchResultOut{
			DictamenID:   r.DictamenID,
			NDictamen:    r.NDictamen,
			FechaDoc:     r.FechaDoc,
			Materia:      r.Materia,
			Descriptores: r.Descriptores,
			Criterio:     r.Criterio,
			Origen:       r.Origen,
			Caracter:     r.Caracter,
			URL:          r.URL,
			PDFURL:       r.PDFURL,
		})
	}
	return SearchCgrDictamenesOutput{
		Results: results,
		Pagination: CgrSearchPaginationOut{
			Total:      result.Pagination.Total,
			Page:       result.Pagination.Page,
			PageSize:   result.Pagination.PageSize,
			TotalPages: result.Pagination.TotalPages,
			HasMore:    result.Pagination.HasMore,
		},
	}
}

func formatCgrSearchResults(result cgr.SearchResponse, args SearchCgrDictamenesArgs) string {
	var b strings.Builder
	total := result.Pagination.Total
	page := result.Pagination.Page
	pageSize := result.Pagination.PageSize
	totalPages := result.Pagination.TotalPages
	if len(result.Results) == 0 {
		if total == 0 {
			fmt.Fprintf(&b, "No dictámenes found for query %q.\n", args.Query)
		} else {
			fmt.Fprintf(&b, "No more results for query %q (page %d beyond total %d, %d pages).\n", args.Query, page, total, totalPages)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "Found %d dictámenes for query %q (page %d/%d, %d per page, total %d)%s\n\n",
		len(result.Results), args.Query, page, totalPages, pageSize, total,
		func() string {
			if total == 10000 {
				return " — more than 10,000 results, refine your search"
			}
			return ""
		}(),
	)
	for i, r := range result.Results {
		fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.DictamenID, r.FechaDoc, truncate(r.Materia, 200))
		if r.Descriptores != "" {
			fmt.Fprintf(&b, "   Descriptores: %s\n", r.Descriptores)
		}
		if r.Criterio != "" {
			fmt.Fprintf(&b, "   Criterio: %s\n", r.Criterio)
		}
		fmt.Fprintf(&b, "   Ver: %s\n", r.URL)
		fmt.Fprintf(&b, "   PDF: %s\n", r.PDFURL)
	}
	if result.Pagination.HasMore {
		fmt.Fprintf(&b, "\nMore results available — call search_cgr_dictamenes with page %d.\n", page+1)
	}
	fmt.Fprintf(&b, "\nUse dictamen_id with get_cgr_dictamen for the full document.\n")
	return b.String()
}
