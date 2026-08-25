package server

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/version"
)

// unsetEnv removes the given env vars so tests get clean defaults.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("Unsetenv(%q): %v", k, err)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	unsetEnv(t, "MCP_TRANSPORT", "MCP_HOST", "MCP_PORT",
		"MCP_PATH", "MCP_AUTH_TOKEN")

	cfg := LoadConfig()

	if cfg.Transport != "http" {
		t.Errorf("Transport = %q, want %q", cfg.Transport, "http")
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != "8000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8000")
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty", cfg.Path)
	}
	if cfg.AuthToken != "" {
		t.Errorf("AuthToken = %q, want empty", cfg.AuthToken)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	unsetEnv(t, "MCP_TRANSPORT", "MCP_HOST", "MCP_PORT",
		"MCP_PATH", "MCP_AUTH_TOKEN")
	env := map[string]string{
		"MCP_TRANSPORT":  "stdio",
		"MCP_HOST":       "0.0.0.0",
		"MCP_PORT":       "9000",
		"MCP_PATH":       "/mcp-custom",
		"MCP_AUTH_TOKEN": "secret",
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Setenv(%q): %v", k, err)
		}
	}
	t.Cleanup(func() {
		unsetEnv(t, "MCP_TRANSPORT", "MCP_HOST", "MCP_PORT",
			"MCP_PATH", "MCP_AUTH_TOKEN")
	})

	cfg := LoadConfig()

	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", cfg.Transport, "stdio")
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9000")
	}
	if cfg.Path != "/mcp-custom" {
		t.Errorf("Path = %q, want %q", cfg.Path, "/mcp-custom")
	}
	if cfg.AuthToken != "secret" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "secret")
	}
}

func TestServerReportsVersion(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = "v9.9.9"
	srv := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	got := serverVersionViaInitialize(t, srv)
	if got != "9.9.9" {
		t.Errorf("Version = %q, want %q (trimmed from v9.9.9)", got, "9.9.9")
	}
}

func TestServerReportsVersionNormalization(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want string
	}{
		{"with v prefix", "v1.2.3", "1.2.3"},
		{"without v", "1.2.3", "1.2.3"},
		{"dev fallback", "dev", "dev"},
		{"with spaces and v", " v2.0.0 ", "2.0.0"},
		{"empty after trim", "v", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orig := version.Version
			t.Cleanup(func() { version.Version = orig })
			version.Version = tc.set
			srv := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
			got := serverVersionViaInitialize(t, srv)
			if got != tc.want {
				t.Errorf("Version = %q, want %q (from %q)", got, tc.want, tc.set)
			}
		})
	}
}

func TestServerVersionConsistentAcrossTransports(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })
	version.Version = "v3.1.4"
	// Same binary, same version, regardless of transport concept.
	srv1 := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	srv2 := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	v1 := serverVersionViaInitialize(t, srv1)
	v2 := serverVersionViaInitialize(t, srv2)
	if v1 != v2 {
		t.Errorf("versions differ across servers: %q vs %q", v1, v2)
	}
	if v1 != "3.1.4" {
		t.Errorf("Version = %q, want %q", v1, "3.1.4")
	}
}

func serverVersionViaInitialize(t *testing.T, srv *mcp.Server) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cTrans, sTrans := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, sTrans, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, cTrans, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil || ir.ServerInfo == nil {
		t.Fatalf("InitializeResult or ServerInfo is nil")
	}
	return ir.ServerInfo.Version
}
