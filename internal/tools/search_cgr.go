package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// SearchCgrArgs carries the arguments of the generic search_cgr tool.
type SearchCgrArgs struct {
	Query       string `json:"query" jsonschema:"search text, e.g. \"quillota\" (empty lists recent)"`
	ExactSearch bool   `json:"exact_search,omitempty" jsonschema:"exact match (default false)"`
	Order       string `json:"order,omitempty" jsonschema:"result order: date (newest), dateasc, score"`
	Page        int    `json:"page,omitempty" jsonschema:"page 1..500, 20 per page"`
	Source      string `json:"source,omitempty" jsonschema:"source: dictamenes|instructivos|contable|auditoria|legislacion|cuentas|consolidados|web|todos (default dictamenes)"`
}

// CgrSearchResultOut is the union projection for all CGR sources.
// Generic dictamen fields are always populated; source-specific fields are
// populated when the source provides them (omitempty keeps other sources clean).
type CgrSearchResultOut struct {
	DictamenID        string `json:"dictamen_id,omitempty"`
	NDictamen         string `json:"n_dictamen,omitempty"`
	FechaDoc          string `json:"fecha_documento,omitempty"`
	Materia           string `json:"materia,omitempty"`
	Descriptores      string `json:"descriptores,omitempty"`
	Criterio          string `json:"criterio,omitempty"`
	Origen            string `json:"origen,omitempty"`
	Caracter          string `json:"caracter,omitempty"`
	URL               string `json:"url,omitempty"`
	PDFURL            string `json:"pdf_url,omitempty"`
	Tipo              string `json:"tipo,omitempty"`
	Numero            string `json:"numero,omitempty"`
	NormativaContable string `json:"normativa_contable,omitempty"`
	Parte             string `json:"parte,omitempty"`
	Destinatarios     string `json:"destinatarios,omitempty"`
	Nombre            string `json:"nombre,omitempty"`
	Resena            string `json:"resena,omitempty"`
	NumeroSentencia   string `json:"numero_sentencia,omitempty"`
	Organismo         string `json:"organismo,omitempty"`
	Materias          string `json:"materias,omitempty"`
	NumericID         string `json:"numeric_doc_id,omitempty"`
}

// CgrSearchPaginationOut is the pagination block for CGR search.
type CgrSearchPaginationOut struct {
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// SearchCgrOutput is the structured content of search_cgr.
type SearchCgrOutput struct {
	Results    []CgrSearchResultOut   `json:"results"`
	Pagination CgrSearchPaginationOut `json:"pagination"`
}

// SearchCgrDictamenesOutput is kept for backward compatibility: alias to generic.
type SearchCgrDictamenesOutput = SearchCgrOutput

var allowedSearchSources = []string{"dictamenes", "instructivos", "contable", "auditoria", "legislacion", "cuentas", "consolidados", "web", "todos"}

// RegisterSearchCgr registers the generic search_cgr tool.
func RegisterSearchCgr(srv *mcp.Server, client cgr.CgrClient) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_cgr",
		Description: "Search Contraloría by source: dictamenes|instructivos|contable|auditoria|legislacion|cuentas|consolidados|web|todos. Returns paginated 20/page with source-specific fields and citation URLs. Use get_cgr_* to fetch full document. Supports order date/dateasc/score, exact_search, page 1..500.",
	}, makeSearchCgr(client))
}

func makeSearchCgr(client cgr.CgrClient) mcp.ToolHandlerFor[SearchCgrArgs, SearchCgrOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchCgrArgs) (*mcp.CallToolResult, SearchCgrOutput, error) {
		order := args.Order
		if order == "" {
			order = "date"
		}
		if order != "date" && order != "dateasc" && order != "score" {
			return errorResult(fmt.Sprintf("order must be date, dateasc or score (got %q)", args.Order)), SearchCgrOutput{}, nil
		}
		page := args.Page
		if page == 0 {
			page = 1
		}
		if page < 1 || page > 500 {
			if page < 1 {
				return errorResult("page must be >= 1"), SearchCgrOutput{}, nil
			}
			return errorResult("page must be <= 500"), SearchCgrOutput{}, nil
		}
		source := strings.ToLower(strings.TrimSpace(args.Source))
		if source == "" {
			source = "dictamenes"
		}
		if !slices.Contains(allowedSearchSources, source) {
			return errorResult(fmt.Sprintf("invalid source %q: must be one of dictamenes, instructivos, contable, auditoria, legislacion, cuentas, consolidados, web, todos", args.Source)), SearchCgrOutput{}, nil
		}
		query := args.Query
		if runes := []rune(query); len(runes) > 500 {
			query = string(runes[:500])
		}
		params := cgr.SearchParams{
			Query:       query,
			ExactSearch: args.ExactSearch,
			Order:       order,
			Page:        page,
			Source:      source,
		}
		result, err := client.Search(ctx, params)
		if err != nil {
			return errorResult(fmt.Sprintf("search cgr failed: %v", err)), SearchCgrOutput{}, nil
		}
		output := buildSearchCgrOutput(result, source)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatSearchCgrResults(result, args)},
			},
		}, output, nil
	}
}

func buildSearchCgrOutput(result cgr.SearchResponse, source string) SearchCgrOutput {
	results := make([]CgrSearchResultOut, 0, len(result.Results))
	src := strings.ToLower(strings.TrimSpace(source))
	for _, r := range result.Results {
		out := CgrSearchResultOut{
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
			NumericID:    r.NumericID,
		}
		switch src {
		case "contable":
			out.Tipo = r.Descriptores
			out.Numero = r.NDictamen
			out.NormativaContable = r.Criterio
			out.Parte = r.Materia
		case "auditoria":
			out.Tipo = r.Descriptores
			out.Numero = r.NDictamen
			out.Nombre = r.Materia
		case "consolidados":
			out.Numero = r.NDictamen
			out.Nombre = r.Materia
			out.Resena = r.Descriptores
			out.Tipo = r.Criterio
		case "cuentas":
			out.NumeroSentencia = r.NDictamen
		case "legislacion":
			out.Tipo = r.Descriptores
			out.Numero = r.NDictamen
			out.Organismo = r.Origen
			out.Materias = r.Materia
		}
		results = append(results, out)
	}
	return SearchCgrOutput{
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

func formatSearchCgrResults(result cgr.SearchResponse, args SearchCgrArgs) string {
	var b strings.Builder
	source := args.Source
	if source == "" {
		source = "dictamenes"
	}
	source = strings.ToLower(strings.TrimSpace(source))
	total := result.Pagination.Total
	page := result.Pagination.Page
	pageSize := result.Pagination.PageSize
	totalPages := result.Pagination.TotalPages
	hasMore := result.Pagination.HasMore
	query := args.Query
	if runes := []rune(query); len(runes) > 500 {
		query = string(runes[:500])
	}
	if len(result.Results) == 0 {
		if total == 0 {
			fmt.Fprintf(&b, "No %s results for query %q.\n", source, query)
		} else {
			fmt.Fprintf(&b, "No more results for query %q (page %d beyond total %d, %d pages).\n", query, page, total, totalPages)
		}
		return b.String()
	}
	capWarn := ""
	if total == 10000 {
		capWarn = " — more than 10,000 results, refine your search"
	}
	fmt.Fprintf(&b, "Search %s %q total %d page %d/%d — has_more:%t%s\n\n", source, query, total, page, totalPages, hasMore, capWarn)
	fmt.Fprintf(&b, "Found %d %s for query %q (page %d/%d, %d per page, total %d)%s\n\n",
		len(result.Results), source, query, page, totalPages, pageSize, total, capWarn)

	for i, r := range result.Results {
		switch source {
		case "contable":
			fmt.Fprintf(&b, "%d. %s — %s — %s %s — %s\n", i+1, r.DictamenID, r.FechaDoc, r.Descriptores, r.NDictamen, truncate(r.Materia, 200))
			if r.Criterio != "" {
				fmt.Fprintf(&b, "   Normativa: %s\n", r.Criterio)
			}
			if r.Origen != "" {
				fmt.Fprintf(&b, "   Origen: %s\n", r.Origen)
			}
		case "auditoria":
			fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.DictamenID, r.FechaDoc, truncate(r.Materia, 200))
			if r.Descriptores != "" {
				fmt.Fprintf(&b, "   Tipo: %s\n", r.Descriptores)
			}
			if r.Criterio != "" {
				fmt.Fprintf(&b, "   Objetivo: %s\n", truncate(r.Criterio, 200))
			}
			if r.Origen != "" {
				fmt.Fprintf(&b, "   Unidad: %s\n", r.Origen)
			}
		case "consolidados":
			fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.NDictamen, r.FechaDoc, truncate(r.Materia, 200))
			if r.Descriptores != "" {
				fmt.Fprintf(&b, "   Reseña: %s\n", truncate(r.Descriptores, 200))
			}
		case "cuentas":
			fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.DictamenID, r.FechaDoc, truncate(r.Materia, 200))
			if r.NDictamen != "" {
				fmt.Fprintf(&b, "   Sentencia: %s\n", r.NDictamen)
			}
		case "legislacion":
			fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.DictamenID, r.FechaDoc, truncate(r.Materia, 200))
			if r.Descriptores != "" {
				fmt.Fprintf(&b, "   Tipo: %s\n", r.Descriptores)
			}
			if r.Origen != "" {
				fmt.Fprintf(&b, "   Organismo: %s\n", r.Origen)
			}
		default:
			fmt.Fprintf(&b, "%d. %s — %s — %s\n", i+1, r.DictamenID, r.FechaDoc, truncate(r.Materia, 200))
			if r.Descriptores != "" {
				fmt.Fprintf(&b, "   Descriptores: %s\n", r.Descriptores)
			}
			if r.Criterio != "" {
				fmt.Fprintf(&b, "   Criterio: %s\n", r.Criterio)
			}
		}
		if r.URL != "" {
			fmt.Fprintf(&b, "   Ver: %s\n", r.URL)
		}
		if r.PDFURL != "" && r.PDFURL != r.URL {
			fmt.Fprintf(&b, "   PDF: %s\n", r.PDFURL)
		} else if r.PDFURL != "" && source == "dictamenes" {
			fmt.Fprintf(&b, "   PDF: %s\n", r.PDFURL)
		}
	}

	if hasMore {
		fmt.Fprintf(&b, "\nMore results available — call search_cgr with source %q and page %d.\n", source, page+1)
	}
	switch source {
	case "contable":
		fmt.Fprintf(&b, "\nUse doc_id/numero with get_cgr_contable for the full document.\n")
	case "auditoria":
		fmt.Fprintf(&b, "\nUse doc_id with get_cgr_auditoria for the full document.\n")
	case "consolidados":
		fmt.Fprintf(&b, "\nUse numero with get_cgr_consolidado for the full document.\n")
	case "cuentas":
		fmt.Fprintf(&b, "\nUse doc_id with get_cgr_cuenta for the full document.\n")
	case "legislacion":
		fmt.Fprintf(&b, "\nUse doc_id with get_cgr_legislacion for the full document.\n")
	case "instructivos":
		fmt.Fprintf(&b, "\nUse dictamen_id with get_cgr_instructivo for the full document.\n")
	default:
		fmt.Fprintf(&b, "\nUse dictamen_id with get_cgr_dictamen for the full document.\n")
	}
	return b.String()
}
