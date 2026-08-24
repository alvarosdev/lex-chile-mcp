BINARY := bin/lex-chile-mcp
IMAGE := lex-chile-mcp:local
CONTAINER := lex-chile-mcp
VERSION ?= $(shell cat VERSION 2>/dev/null | tr -d ' \n' | sed 's/^v//')
LDFLAGS := -s -w -X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=$(VERSION)
# Temporary Bearer token for local testing (make run-http-auth).
DEV_AUTH_TOKEN ?= devtoken
# podman is the main container runtime; docker compose is the fallback.
COMPOSE := $(shell command -v podman-compose >/dev/null 2>&1 && echo podman-compose || echo docker compose)

.DEFAULT_GOAL := help

.PHONY: help build run-http run-http-auth run-stdio test vet fmt fmt-check mock check vuln \
	podman-build podman-run podman-stop podman-logs compose-up compose-down smoke dist clean

help: ## Show this help (default target)
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build the server binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/lex-chile-mcp

run-http: ## Run the server (HTTP, default config, no auth)
	go run -ldflags="$(LDFLAGS)" ./cmd/lex-chile-mcp

run-http-auth: ## Run the server (HTTP) with a temporary Bearer token for testing (default: devtoken, override: make run-http-auth DEV_AUTH_TOKEN=x)
	MCP_AUTH_TOKEN=$(DEV_AUTH_TOKEN) go run -ldflags="$(LDFLAGS)" ./cmd/lex-chile-mcp

run-stdio: ## Run the server over stdio
	MCP_TRANSPORT=stdio go run -ldflags="$(LDFLAGS)" ./cmd/lex-chile-mcp
test: ## Run all tests (no cache)
	go test ./... -count=1

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w ./cmd ./internal

fmt-check: ## Fail if any Go file is not gofmt-clean
	@files=$$(gofmt -l ./cmd ./internal); \
	if [ -n "$$files" ]; then \
		echo "files need formatting:"; echo "$$files"; exit 1; \
	fi

mock: ## Regenerate mocks (mockery, declared as a Go tool)
	go tool mockery

vuln: ## Scan dependencies for known vulnerabilities (govulncheck, pinned)
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: ## Full local verification, same as CI: build + vet + test
	$(MAKE) build vet test

clean: ## Remove bin/ and dist/
	rm -rf bin && \
	rm -rf dist

podman-build: ## Build the container image with podman
	podman build -t $(IMAGE) .

podman-run: podman-build ## Run the image with podman (port 8000, health at /health)
	-podman rm -f $(CONTAINER)
	podman run -d --name $(CONTAINER) -p 8000:8000 \
		--read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges \
		-e MCP_AUTH_TOKEN=$${MCP_AUTH_TOKEN:-} \
		$(IMAGE)
	@echo "Container $(CONTAINER) running — health: curl http://localhost:8000/health"

podman-stop: ## Stop and remove the podman container
	-podman rm -f $(CONTAINER)

podman-logs: ## Tail the container logs
	podman logs -f $(CONTAINER)

compose-up: ## Start via podman-compose (fallback: docker compose)
	$(COMPOSE) up -d

compose-down: ## Stop and remove via the compose fallback chain
	$(COMPOSE) down

smoke: ## Run the smoke test (requires the server: make run-http)
	bash scripts/smoke.sh

dist: ## Build the cross-platform distributions into dist.zip (local = CI)
	bash scripts/build-dist.sh 0.0.0-local
