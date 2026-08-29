package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// GetLawHistoryArgs carries the arguments of the get_law_history tool.
type GetLawHistoryArgs struct {
	NormID int64 `json:"norm_id" jsonschema:"the norm id (norm_id) from search_laws results"`
}

// GetLawHistoryOutput is the structured content of get_law_history.
//
// MUST be an object type: MCP requires outputSchema to describe an object
// (SEP-2106) — a top-level slice would generate "type": "array" and strict
// clients reject tools/list.
type GetLawHistoryOutput struct {
	Groups []bcn.HistoriaGrupo `json:"groups"`
}

// RegisterGetLawHistory registers the get_law_history tool on the MCP server.
func RegisterGetLawHistory(srv *mcp.Server, client bcn.LawClient) {
	registerTool(srv, &mcp.Tool{
		Name: "get_law_history",
		Description: "Get the legislative history of a Chilean law by its norm_id: its own " +
			"history, the laws that modified it (modificatorias) and the laws it modified " +
			"(modificadas). Each entry carries the date, description, summary and the " +
			"LeyChile link of the record's norm.",
	}, makeGetLawHistory(client))
}

func makeGetLawHistory(client bcn.LawClient) mcp.ToolHandlerFor[GetLawHistoryArgs, GetLawHistoryOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetLawHistoryArgs) (*mcp.CallToolResult, GetLawHistoryOutput, error) {
		if args.NormID <= 0 {
			return errorResult("norm_id must be a positive number"), GetLawHistoryOutput{}, nil
		}

		grupos, err := client.GetLawHistory(ctx, args.NormID)
		if err != nil {
			return errorResult(fmt.Sprintf("get law history failed: %v", err)), GetLawHistoryOutput{}, nil
		}
		if len(grupos) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("No legislative history found for norm_id %d.", args.NormID)},
				},
			}, GetLawHistoryOutput{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatHistoria(grupos)},
			},
		}, GetLawHistoryOutput{Groups: grupos}, nil
	}
}

// formatHistoria renders the history groups for the LLM. LeyChile ficha
// links are ALWAYS built from id_norma_hl (the idNorma of the norm the
// record belongs to) — never from id_norma (the related norm) nor from the
// number inside the history URL (Historia ID).
func formatHistoria(grupos []bcn.HistoriaGrupo) string {
	var b strings.Builder
	for _, g := range grupos {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", g.Titulo, g.TipoDesc)
		for _, h := range g.Hls {
			fmt.Fprintf(&b, "- %s — %s\n", h.Fecha, h.Descripcion)
			if h.Bajada != "" {
				fmt.Fprintf(&b, "  %s\n", h.Bajada)
			}
			fmt.Fprintf(&b, "  Ficha: https://www.leychile.cl/Navegar?idNorma=%d\n", h.IDNormaHL)
			if h.Enlace != "" {
				fmt.Fprintf(&b, "  History: %s\n", h.Enlace)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
