package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// CountCgrJurisprudenciaArgs carries the arguments of the count_cgr_jurisprudencia tool.
type CountCgrJurisprudenciaArgs struct {
	Query       string `json:"query" jsonschema:"search text, e.g. \"quillota\" or \"bono\" (empty counts all)"`
	ExactSearch bool   `json:"exact_search,omitempty" jsonschema:"exact match (default false)"`
}

// CountCgrBucketOut is one bucket in the structured output.
type CountCgrBucketOut struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// CountCgrJurisprudenciaOutput is the structured content of count_cgr_jurisprudencia.
type CountCgrJurisprudenciaOutput struct {
	Query   string              `json:"query"`
	Total   int                 `json:"total"`
	Buckets []CountCgrBucketOut `json:"buckets"`
}

// RegisterCountCgrJurisprudencia registers the count_cgr_jurisprudencia tool.
func RegisterCountCgrJurisprudencia(srv *mcp.Server, client cgr.CgrClient) {
	registerTool(srv, &mcp.Tool{
		Name:        "count_cgr_jurisprudencia",
		Description: "Count cross-type Contraloría results (dictamenes, auditoria, legislacion, etc.) for a query without fetching documents. Use it to explore how many results exist per type before searching. Returns total and buckets with counts per type.",
	}, makeCountCgrJurisprudencia(client))
}

func makeCountCgrJurisprudencia(client cgr.CgrClient) mcp.ToolHandlerFor[CountCgrJurisprudenciaArgs, CountCgrJurisprudenciaOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CountCgrJurisprudenciaArgs) (*mcp.CallToolResult, CountCgrJurisprudenciaOutput, error) {
		result, err := client.CountJurisprudencia(ctx, args.Query, args.ExactSearch)
		if err != nil {
			return errorResult(fmt.Sprintf("count jurisprudencia failed: %v", err)), CountCgrJurisprudenciaOutput{}, nil
		}
		output := CountCgrJurisprudenciaOutput{
			Query: result.Query,
			Total: result.Total,
		}
		for _, b := range result.Buckets {
			output.Buckets = append(output.Buckets, CountCgrBucketOut{Type: b.Type, Count: b.Count})
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: formatCount(result)},
			},
		}, output, nil
	}
}

func formatCount(c cgr.CountResponse) string {
	var b strings.Builder
	if len(c.Buckets) == 0 {
		fmt.Fprintf(&b, "No results for query %q (total 0).\n", c.Query)
		return b.String()
	}
	fmt.Fprintf(&b, "Query %q: %d results — ", c.Query, c.Total)
	parts := make([]string, 0, len(c.Buckets))
	for _, bk := range c.Buckets {
		parts = append(parts, fmt.Sprintf("%s %d", bk.Type, bk.Count))
	}
	b.WriteString(strings.Join(parts, ", "))
	b.WriteString("\n")
	return b.String()
}
