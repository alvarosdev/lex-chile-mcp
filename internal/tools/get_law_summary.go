package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawSummaryArgs carries the arguments of the get_law_summary tool.
// VersionDate is optional (omitempty keeps it out of the required schema).
type GetLawSummaryArgs struct {
	NormID      int64  `json:"norm_id" jsonschema:"the norm id (norm_id) from search_laws results"`
	VersionDate string `json:"version_date,omitempty" jsonschema:"version in force at this date (YYYY-MM-DD, optional — defaults to the latest version)"`
}

// RegisterGetLawSummary registers the get_law_summary tool on the MCP server.
func RegisterGetLawSummary(srv *mcp.Server, client bcn.LawClient) {
	registerTool(srv, &mcp.Tool{
		Name: "get_law_summary",
		Description: "Get a lightweight overview of a Chilean law, decree or resolution by its " +
			"norm_id (from search_laws): title, source, matters, norm categories, the " +
			"official BCN summary, the table of contents with the section ids, and the size " +
			"of the full text. Use it FIRST to understand what a norm is about and to decide " +
			"which sections to read with get_law(section_id=...) — avoid requesting the full " +
			"content of long norms.",
	}, makeGetLawSummary(client))
}

func makeGetLawSummary(client bcn.LawClient) mcp.ToolHandlerFor[GetLawSummaryArgs, bcn.NormaSummary] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetLawSummaryArgs) (*mcp.CallToolResult, bcn.NormaSummary, error) {
		if args.NormID <= 0 {
			return errorResult("norm_id must be a positive number"), bcn.NormaSummary{}, nil
		}
		if err := validateVersionDate(args.VersionDate); err != nil {
			return errorResult(err.Error()), bcn.NormaSummary{}, nil
		}

		query := bcn.NormaQuery{NormID: args.NormID, VersionDate: args.VersionDate}
		summary, err := client.GetNormaSummary(ctx, query)
		if err != nil {
			if errors.Is(err, bcn.ErrNormaNotFound) {
				return errorResult(fmt.Sprintf("norma not found: norm_id %d does not exist in LeyChile", args.NormID)), bcn.NormaSummary{}, nil
			}
			return errorResult(fmt.Sprintf("get law summary failed: %v", err)), bcn.NormaSummary{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatNormaSummary(summary, args.VersionDate)},
			},
		}, summary, nil
	}
}

// formatNormaSummary renders the summary for the LLM: the map of the law.
// The official BCN summary is short by nature, so it goes complete in the
// text view. The size line states the magnitude of the FULL norm (the
// summary counts are always the whole document), and the structure list is
// the FOLDED TOC — containers with their section ids, sizes and textual
// article ranges — so a meganorm map stays in a few thousand tokens while
// still locating every article by range.
func formatNormaSummary(s bcn.NormaSummary, versionDate string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.TituloNorma)
	if versionDate != "" {
		fmt.Fprintf(&b, "Version: as of %s\n", versionDate)
	}
	fmt.Fprintf(&b, "Source: %s\n", s.Fuente)
	if len(s.Materias) > 0 {
		fmt.Fprintf(&b, "Matters: %s\n", strings.Join(s.Materias, ", "))
	}
	if len(s.CategoriasNorma) > 0 {
		fmt.Fprintf(&b, "Categories: %s\n", strings.Join(s.CategoriasNorma, ", "))
	}
	fmt.Fprintf(&b, "Size: %s chars · %s\n", humanCount(s.CharCount), formatArticles(s.ArticleCount))
	for _, r := range s.Resumenes {
		fmt.Fprintf(&b, "\nSummary:\n%s\n", r)
	}

	b.WriteString("\n## Structure\n")
	renderFoldedTOC(&b, s.Estructura, s.SectionSizes)
	return b.String()
}
