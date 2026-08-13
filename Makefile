# One entry point for both halves of the project. Run every target from here.
#
# Settings come from api/.env, which is not committed. Run make setup once on a
# fresh clone to write it.

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
SQLC  := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
OAPI  := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
WEB_PORT ?= 3000

-include api/.env
export

.DEFAULT_GOAL := help

.PHONY: help
help: ## List every target
	@grep -hE '^[a-z][a-z-]*:.*##' $(MAKEFILE_LIST) \
		| sort \
		| awk -F':.*## ' '{printf "  \033[1m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: setup
setup: ## Get a fresh clone ready to run
	@test -f api/.env || { cp api/.env.example api/.env; \
		echo "Wrote api/.env from the example. Check the database URLs in it."; }
	cd web && bun install
	$(MAKE) migrate migrate-test

# Running

.PHONY: api
api: need-db ## Run the API
	cd api && go run ./cmd/server

.PHONY: web
web: ## Run the site
	cd web && PORT=$(WEB_PORT) bun run dev

# Checking

.PHONY: check
check: fmt-check vet test lint ## Everything CI would run

.PHONY: test
test: need-test-db ## Run the Go tests
# -p 1 because every package sharing the test database empties it on the way in.
	cd api && go test -p 1 ./...

.PHONY: cover
cover: need-test-db ## Report Go test coverage per package
	cd api && go test -p 1 -cover ./...

.PHONY: vet
vet: ## Report suspicious Go code
	cd api && go vet ./...

.PHONY: lint
lint: ## Check the site with Biome and the TypeScript compiler
	cd web && bun run lint
	cd web && bunx tsc --noEmit

.PHONY: fmt
fmt: ## Format Go and site sources in place
	cd api && gofmt -w .
	cd web && bun run format

.PHONY: fmt-check
fmt-check: ## Fail if any Go file needs formatting
	@unformatted=$$(cd api && gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "needs gofmt:"; echo "$$unformatted"; exit 1; fi

# Database

.PHONY: migrate
migrate: need-db ## Apply migrations to the dev database
	cd api && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up

.PHONY: migrate-test
migrate-test: need-test-db ## Apply migrations to the test database
	cd api && $(GOOSE) -dir migrations postgres "$(TEST_DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: need-db ## Roll the dev database back one migration
	cd api && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" down

.PHONY: migrate-status
migrate-status: need-db ## Show which migrations have run
	cd api && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" status

# Generated code, never hand edited

.PHONY: generate
generate: ## Regenerate database code, server stubs, and the site's API types
	cd api && $(SQLC) generate
	cd api/openapi && $(OAPI) -config cfg-server.yaml openapi.yaml
	cd web && bun run gen:api

# Guards

.PHONY: need-db need-test-db
need-db:
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set. Run make setup."; exit 1; }

need-test-db:
	@test -n "$(TEST_DATABASE_URL)" || { echo "TEST_DATABASE_URL is not set. Run make setup."; exit 1; }
