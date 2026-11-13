SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

SERVICES := api collector flightcheck ingest insights alerts
GO_MODULES := $(addprefix services/,$(SERVICES)) services/internal/platform services/internal/db cli sdks/go
BIN_DIR := bin

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Format ───────────────────────────────────────────────────────────────────
.PHONY: fmt
fmt: fmt-go fmt-ts fmt-py ## Format all code

.PHONY: fmt-go
fmt-go:
	gofmt -l -w .
	@for mod in $(GO_MODULES); do \
		(cd $$mod && goimports -l -w . 2>/dev/null || true); \
	done

.PHONY: fmt-ts
fmt-ts:
	pnpm prettier --write "packages/**/*.ts"

.PHONY: fmt-py
fmt-py:
	cd sdks/python && uv run ruff format .

# ── Lint ─────────────────────────────────────────────────────────────────────
.PHONY: lint
lint: lint-go lint-ts lint-py ## Lint all code

.PHONY: lint-go
lint-go:
	@for mod in $(GO_MODULES); do \
		echo "→ lint $$mod"; \
		(cd $$mod && golangci-lint run --timeout 5m ./...); \
	done

.PHONY: lint-ts
lint-ts:
	pnpm turbo run lint

.PHONY: lint-py
lint-py:
	cd sdks/python && uv run ruff check . && uv run mypy vigil

# ── Test ─────────────────────────────────────────────────────────────────────
.PHONY: test
test: test-go test-ts test-py ## Run all tests

.PHONY: test-go
test-go:
	@for mod in $(GO_MODULES); do \
		echo "→ testing $$mod"; \
		(cd $$mod && go test -short -race -count=1 -timeout=120s ./...); \
	done

.PHONY: test-ts
test-ts:
	pnpm turbo run test

.PHONY: test-py
test-py:
	cd sdks/python && uv run pytest -v; ec=$$?; [ $$ec -eq 5 ] && exit 0 || exit $$ec

# ── Build ────────────────────────────────────────────────────────────────────
.PHONY: build
build: build-go build-ts ## Build all binaries and packages

.PHONY: build-go
build-go:
	mkdir -p $(BIN_DIR)
	@for svc in $(SERVICES); do \
		echo "→ building $$svc"; \
		go build -o $(BIN_DIR)/vigil-$$svc ./services/$$svc/cmd/$$svc; \
	done
	go build -o $(BIN_DIR)/vigil ./cli/cmd/vigil
	go build -o $(BIN_DIR)/vigil-migrate ./services/internal/db/cmd/migrate

.PHONY: build-ts
build-ts:
	pnpm turbo run build

# ── Docker ───────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build: ## Build all service Docker images
	@for svc in $(SERVICES); do \
		echo "→ docker build $$svc"; \
		docker build -f deploy/docker/Dockerfile.$$svc -t vigil-$$svc:dev .; \
	done

.PHONY: up
up: ## Start local stack (docker compose)
	docker compose -f deploy/docker/docker-compose.yml up -d

.PHONY: down
down: ## Stop local stack
	docker compose -f deploy/docker/docker-compose.yml down

# ── Migrations ───────────────────────────────────────────────────────────────
.PHONY: migrate
migrate: ## Apply all Postgres + ClickHouse migrations
	go run ./services/internal/db/cmd/migrate -direction=up

.PHONY: migrate-down
migrate-down: ## Roll back one migration step
	go run ./services/internal/db/cmd/migrate -direction=down

.PHONY: seed
seed: ## Seed demo data (run after migrate)
	go run ./services/internal/db/cmd/seed

# ── Integration tests ────────────────────────────────────────────────────────
.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	cd services/internal/db && go test -v -race -count=1 -timeout=300s ./integration/...
	cd services/collector && go test -v -race -count=1 -timeout=300s ./integration/...
	cd services/flightcheck && go test -v -race -count=1 -timeout=300s ./integration/...

# ── CI shortcuts ─────────────────────────────────────────────────────────────
.PHONY: ci
ci: fmt lint test build ## Run the full CI pipeline locally
