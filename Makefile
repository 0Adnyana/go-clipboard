SERVER_BIN := ./tmp/server

# Prefer a copy on the PATH and fall back to the Go install location, so a tool
# installed anywhere PATH can see it is used, while `go install` defaults still
# work on a machine that never added GOPATH/bin to PATH.
GOBIN := $(shell go env GOPATH)/bin
SQLC := $(shell command -v sqlc 2>/dev/null || echo $(GOBIN)/sqlc)
AIR := $(shell command -v air 2>/dev/null || echo $(GOBIN)/air)
OAPI_CODEGEN := $(shell command -v oapi-codegen 2>/dev/null || echo $(GOBIN)/oapi-codegen)

OPENAPI_SPEC := docs/api/openapi.yaml

define RUN_WITH_ENV
set -a && . ./.env && set +a &&
endef

# The database name is written down once, in DATABASE_URL. The query string is
# stripped first because the Homebrew socket form carries host=/tmp, so the last
# slash in the whole URL is not the one preceding the database name.
DB_NAME_FROM_URL = name=$$(basename "$${DATABASE_URL%%\?*}"); [ -n "$$name" ] || { echo "DATABASE_URL does not name a database" >&2; exit 1; };

.PHONY: help db-create db-reset migrate generate generate-sqlc generate-openapi generate-openapi-go generate-openapi-ts test lint dev check-dev-tools

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

db-create: ## Create the database named by DATABASE_URL if it does not already exist
	@$(RUN_WITH_ENV) $(DB_NAME_FROM_URL) createdb "$$name" || true

# dropdb --force is required because a running air process holds pool connections.
db-reset: ## Drop, recreate, and migrate the database named by DATABASE_URL
	@$(RUN_WITH_ENV) $(DB_NAME_FROM_URL) dropdb --force --if-exists "$$name"
	$(MAKE) db-create
	$(MAKE) migrate

migrate: $(SERVER_BIN) ## Apply pending database migrations
	@$(RUN_WITH_ENV) $(SERVER_BIN) migrate up

generate: generate-sqlc generate-openapi ## Regenerate all code from its source of truth (never edit generated files by hand)

generate-sqlc: ## Regenerate sqlc code from db/queries (SQL -> Go Querier)
	$(SQLC) generate

generate-openapi: generate-openapi-go generate-openapi-ts ## Regenerate Go and TypeScript from the OpenAPI contract

generate-openapi-go: ## Generate Go models + net/http ServerInterface from docs/api/openapi.yaml
	$(OAPI_CODEGEN) -config oapi-codegen.yaml $(OPENAPI_SPEC)

generate-openapi-ts: ## Generate TypeScript types from docs/api/openapi.yaml
	@if [ -d web ]; then cd web && pnpm run gen:api; fi

test: ## Run Go and frontend tests
	go test ./...
	@if [ -d web ]; then cd web && pnpm test; fi

lint: ## Lint Go and frontend sources
	gofmt -l . | grep . && exit 1 || true
	go vet ./...
	@if [ -d web ]; then cd web && pnpm lint; fi

check-dev-tools: ## Verify dev prerequisites are on PATH
	@command -v caddy >/dev/null || (echo "caddy not found. Install with: brew install caddy" && exit 1)
	@command -v $(AIR) >/dev/null || (echo "air not found. Install with: go install github.com/air-verse/air@latest" && exit 1)
	@command -v $(SQLC) >/dev/null || (echo "sqlc not found. Install with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest" && exit 1)
	@command -v $(OAPI_CODEGEN) >/dev/null || (echo "oapi-codegen not found. Install with: go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest" && exit 1)

# Vite runs at warn level so it does not advertise its own :5173 URL underneath
# the proxy URL; opening that port directly makes relative /api calls hit Vite,
# which answers with index.html. The cost is that HMR update lines are hidden too.
dev: check-dev-tools $(SERVER_BIN) ## Run Caddy, air, and the Vite dev server behind one proxy URL
	@$(RUN_WITH_ENV) \
	trap 'kill 0' INT TERM; \
	echo "Open http://localhost:$${PROXY_PORT:-3000}"; \
	caddy run --config Caddyfile & \
	$(AIR) & \
	cd web && pnpm dev --logLevel warn & \
	wait

$(SERVER_BIN): $(shell find . -name '*.go' -not -path './web/*')
	go build -o $(SERVER_BIN) ./cmd/server
