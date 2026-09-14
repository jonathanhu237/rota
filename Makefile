GOVULNCHECK_VERSION ?= v1.2.0
GOVULNCHECK_GO_VERSION ?= go@1.27.0

.PHONY: up down logs logs-admin logs-mailpit migrate-up migrate-down-one migrate-status build test api-test api-build api-vet api-integration admin-install admin-dev admin-lint admin-check admin-test admin-build vuln-check

up:
	docker compose --profile development up -d api admin mailpit

down:
	docker compose down

logs:
	docker compose logs -f api

logs-admin:
	docker compose logs -f admin

logs-mailpit:
	docker compose logs -f mailpit

migrate-up:
	docker compose up -d postgres
	docker compose --profile tools run --rm migrate

migrate-down-one:
	docker compose --profile tools run --rm migrate down 1

migrate-status:
	docker compose --profile tools run --rm migrate version

build:
	docker compose build api admin migrate

api-test:
	cd api && go test ./...

api-build:
	cd api && go build ./...

api-vet:
	cd api && go vet ./...

api-integration:
	./scripts/test-integration.sh

admin-install:
	cd admin && pnpm install --ignore-scripts

admin-dev:
	cd admin && pnpm dev

admin-lint:
	cd admin && pnpm lint

admin-check:
	cd admin && pnpm check

admin-test:
	cd admin && pnpm test

admin-build:
	cd admin && pnpm build

vuln-check:
	cd api && mise exec $(GOVULNCHECK_GO_VERSION) -- go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

test: api-vet api-build api-test admin-lint admin-check admin-test admin-build
