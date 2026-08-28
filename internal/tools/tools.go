// Package tools provides MCP tools for the lex-chile-mcp server.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
)

// RegisterTools registers all BCN tools on the MCP server. The law client is
// injected: the same process-wide instance is reused for every request.
func RegisterTools(srv *mcp.Server, client bcn.LawClient) {
	RegisterSearchLaws(srv, client)
	RegisterGetLaw(srv, client)
	RegisterGetLawSummary(srv, client)
	RegisterGetLawHistory(srv, client)
}

// RegisterCgrTools registers all CGR tools on the MCP server.
func RegisterCgrTools(srv *mcp.Server, client cgr.CgrClient) {
	RegisterSearchCgr(srv, client)
	RegisterSearchCgrDictamenes(srv, client)
	RegisterGetCgrDictamen(srv, client)
	RegisterGetCgrInstructivo(srv, client)
	RegisterGetCgrContable(srv, client)
	RegisterGetCgrAuditoria(srv, client)
	RegisterGetCgrConsolidado(srv, client)
	RegisterGetCgrCuenta(srv, client)
	RegisterGetCgrLegislacion(srv, client)
	RegisterCountCgrJurisprudencia(srv, client)
}

// errorResult renders a tool error as an MCP error result. The handler
// still returns nil error: the failure is part of the tool response, so
// the client receives the message instead of a protocol error.
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
