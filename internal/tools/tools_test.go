package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
)

// newTestClient connects an in-memory MCP client to a server with the
// tools registered, and returns the client session. The law client is a
// fresh mock: tools never reach the network in tests.
func newTestClient(t *testing.T, ctx context.Context, lawClient bcn.LawClient) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	RegisterTools(server, lawClient)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect(): %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect(): %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	return clientSession
}
