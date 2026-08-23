# One entry point for both halves of the project. Run every target from here.
#
# Settings come from api/.env, which is not committed. Run make setup once on a
# fresh clone to write it.

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
SQLC  := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
OAPI  := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
ACTIONLINT := go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
WEB_PORT ?= 3000
TEST ?= ./...
VERSION ?=
SERVICE ?=
PROD_ENV ?= /etc/illarin/production.env
NGINX_IMAGE ?= nginx:alpine
NGINX := docker run --rm --network host -v "$(CURDIR):/work:ro" -w /work $(NGINX_IMAGE) \
	nginx -p /work/ -c nginx/local.conf

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
		linking_key=$$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n'); \
		sed -i "s/^LINKING_HMAC_KEY=$$/LINKING_HMAC_KEY=$$linking_key/" api/.env; \
		echo "Wrote api/.env from the example. Check the database URLs in it."; }
	$(MAKE) web-install
	$(MAKE) migrate migrate-test

.PHONY: web-install
web-install: ## Install the site's locked dependencies
	cd web && bun install --frozen-lockfile

# Running

.PHONY: api
api: need-db ## Run the API
	cd api && go run ./cmd/server

.PHONY: web
web: ## Run the site
	cd web && PORT=$(WEB_PORT) bun run dev

.PHONY: proxy
proxy: ## Run the local nginx proxy on port 8000
	$(NGINX) -g 'daemon off;'

# Production

.PHONY: prod-deploy
prod-deploy: ## Deploy one immutable Git commit to production
	@ILLARIN_ENV_FILE="$(PROD_ENV)" VERSION="$(VERSION)" ./ops/deploy.sh

.PHONY: prod-rollback
prod-rollback: ## Restore the previously deployed application images
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/rollback.sh

.PHONY: prod-status
prod-status: ## Show production container and health status
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/compose.sh ps

.PHONY: prod-logs
prod-logs: ## Follow recent production logs; optionally set SERVICE
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/compose.sh logs --tail=200 --follow $(SERVICE)

.PHONY: prod-restart
prod-restart: ## Restart one production service with SERVICE=api, web or gateway
	@test -n "$(SERVICE)" || { echo "Set SERVICE to api, web or gateway."; exit 1; }
	@case "$(SERVICE)" in api|web|gateway) ;; *) echo "SERVICE must be api, web or gateway."; exit 1 ;; esac
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/compose.sh restart "$(SERVICE)"

.PHONY: prod-smoke
prod-smoke: ## Check the production gateway, API and site
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/smoke.sh

.PHONY: prod-config-check
prod-config-check: ## Validate the production Compose configuration
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/compose.sh config --quiet

.PHONY: prod-backup prod-backup-init prod-backup-check
prod-backup: ## Run an off-box backup when backups are enabled
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/backup.sh run

prod-backup-init: ## Create the configured off-box backup repository
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/backup.sh init

prod-backup-check: ## Verify the configured off-box backup repository
	@ILLARIN_ENV_FILE="$(PROD_ENV)" ./ops/backup.sh check

# Checking

.PHONY: check
check: fmt-check vet test test-web lint openapi-check workflow-check ## Everything CI would run

.PHONY: test
test: need-test-db ## Run the Go tests
# -p 1 because every package sharing the test database empties it on the way in.
	cd api && go test -p 1 $(TEST)

.PHONY: test-web
test-web: ## Run the site tests
	cd web && bun test

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

.PHONY: proxy-check
proxy-check: ## Check the local nginx configuration
	$(NGINX) -t

.PHONY: workflow-check
workflow-check: ## Check GitHub Actions workflows and their shell commands
	$(ACTIONLINT)

.PHONY: production-proxy-check
production-proxy-check: ## Check the production nginx configuration
	docker run --rm -v "$(CURDIR)/nginx/production.conf:/etc/nginx/nginx.conf:ro" \
		nginx:1.31.2-alpine3.23 nginx -t

.PHONY: image-api image-web images
image-api: ## Build the production Go image locally
	docker build -f api/Dockerfile -t illarin-api:local .

image-web: ## Build the production Next image locally
	docker build --build-arg ILLARIN_VERSION="$(or $(VERSION),development)" \
		-f web/Dockerfile -t illarin-web:local .

images: image-api image-web ## Build both production application images

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

.PHONY: generate openapi-bundle
generate: openapi-bundle ## Regenerate database code, server stubs, and the site's API types
	cd api && $(SQLC) generate
	cd api/openapi && $(OAPI) -config cfg-server.yaml openapi.gen.yaml
	cd web && bun run gen:api

openapi-bundle: ## Bundle the OpenAPI modules for publication and generation
	cd web && bun run bundle:api

.PHONY: openapi-check
openapi-check: ## Check that the published OpenAPI bundle is current
	cd web && bun run check:api

.PHONY: refractive-assets
refractive-assets: ## Generate the deterministic refractive art assets
	cd web && bun scripts/generate-refractive-assets.mjs

.PHONY: quiet-page-art
quiet-page-art: ## Generate the empty and barren page artwork, one piece per kind
	cd web && bun scripts/generate-quiet-page-art.mjs

.PHONY: archive-cutouts
archive-cutouts: ## Neutralize the archive mascot glass cutouts
	cd web && bun scripts/neutralize-archive-cutouts.mjs

# Guards

.PHONY: need-db need-test-db
need-db:
	@test -n "$(DATABASE_URL)" || { echo "DATABASE_URL is not set. Run make setup."; exit 1; }

need-test-db:
	@test -n "$(TEST_DATABASE_URL)" || { echo "TEST_DATABASE_URL is not set. Run make setup."; exit 1; }
