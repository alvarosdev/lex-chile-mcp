// Package version provides the build-time version string for the MCP server.
// Overridden via -ldflags "-X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=<version>".
package version

// Version is the server version reported in mcp.Implementation.Version.
// Default "dev" when built without ldflags (e.g. go run).
var Version = "dev"
