// Lex Chile MCP Server.
//
// Supports stdio and streamable HTTP transports with optional
// bearer-token authentication.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alvarosdev/lex-chile-mcp/internal/bcn"
	"github.com/alvarosdev/lex-chile-mcp/internal/cgr"
	"github.com/alvarosdev/lex-chile-mcp/internal/config"
	bcnPrompts "github.com/alvarosdev/lex-chile-mcp/internal/prompts/bcn"
	cgrPrompts "github.com/alvarosdev/lex-chile-mcp/internal/prompts/cgr"
	"github.com/alvarosdev/lex-chile-mcp/internal/server"
	"github.com/alvarosdev/lex-chile-mcp/internal/tools"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Read all configuration once at startup — never in hot paths.
	cfg := server.LoadConfig()

	logger.Info("Starting Lex Chile MCP Server",
		"transport", cfg.Transport,
	)

	// Load the API resources contract once at startup — fail fast on an
	// invalid contract. The contract is baked into the binary via go:embed
	// (internal/config/api.resources.yaml), never by hot-reload.
	resources, err := config.LoadEmbedded()
	if err != nil {
		logger.Error("Invalid embedded API resources contract", "error", err)
		os.Exit(1)
	}
	logger.Info("API resources loaded", "resources", len(resources.Resources))

	// Load the curated prompts once at startup — baked via go:embed per domain.
	bcnPS, err := bcnPrompts.LoadEmbedded()
	if err != nil {
		logger.Error("Invalid embedded BCN prompts contract", "error", err)
		os.Exit(1)
	}
	cgrPS, err := cgrPrompts.LoadEmbedded()
	if err != nil {
		logger.Error("Invalid embedded CGR prompts contract", "error", err)
		os.Exit(1)
	}
	logger.Info("Prompts loaded", "bcn", len(bcnPrompts.ToolNames()), "cgr", len(cgrPrompts.ToolNames()))

	// Build the process-wide law client (the injected singleton) and
	// register all tools with it.
	lawClient := bcn.NewClient(resources, logger)
	cgrClient := cgr.NewClient(resources, logger)

	// Create MCP server and register tools and prompts.
	srv := server.New(logger)
	tools.RegisterTools(srv, lawClient)
	tools.RegisterCgrTools(srv, cgrClient)
	bcnPrompts.RegisterPrompts(srv, bcnPS)
	cgrPrompts.RegisterPrompts(srv, cgrPS)
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cfg.Transport {
	case "stdio":
		logger.Info("Starting stdio transport")
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}

	default: // "http", "streamable-http", "sse"
		if err := runHTTP(ctx, logger, srv, cfg); err != nil {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}
}

// runHTTP starts the server with streamable HTTP transport.
func runHTTP(ctx context.Context, logger *slog.Logger, srv *mcp.Server, cfg server.Config) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{},
	)

	// Wrap with auth middleware if a token is configured.
	var mux http.Handler = handler
	if cfg.AuthToken != "" {
		mux = staticTokenAuthMiddleware(cfg.AuthToken)(handler)
		logger.Info("Authentication enabled — Bearer token required")
	}

	// Route: MCP at configured path, health at /health.
	mcpPath := cfg.Path
	if mcpPath == "" {
		mcpPath = "/mcp"
	}
	mux2 := http.NewServeMux()
	mux2.Handle(mcpPath, mux)
	mux2.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux2,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second, // anti-Slowloris: ReadTimeout does not cover header phase
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	logger.Info("Listening", "addr", addr, "path", mcpPath)

	// Graceful shutdown: wait for signal, then drain.
	go func() {
		<-ctx.Done()
		logger.Info("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP shutdown error", "error", err)
		}
	}()

	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// staticTokenAuthMiddleware wraps a handler with bearer-token auth against a
// static env-var token. The TokenInfo carries no Expiration (a static token
// is valid for the process lifetime), so AllowMissingExpiration MUST be set —
// otherwise the SDK rejects every request with "token missing expiration".
func staticTokenAuthMiddleware(token string) func(http.Handler) http.Handler {
	verifier := func(_ context.Context, tokenStr string, _ *http.Request) (*auth.TokenInfo, error) {
		if subtle.ConstantTimeCompare([]byte(tokenStr), []byte(token)) != 1 {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{}, nil
	}
	return auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		AllowMissingExpiration: true,
	})
}
