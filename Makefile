-include .env

export

GOOSE_DRIVER = postgres
GOOSE_DBSTRING = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
GOOSE_MIGRATION_DIR = ./migrations
SCENARIO ?= basic

.PHONY: run-backend run-frontend migrate-up migrate-down migrate-status seed test-backend test-integration prod-up prod-down prod-logs prod-pull

run-backend:
	@cd backend && go run ./cmd/server

run-frontend:
	@cd frontend && pnpm dev

migrate-up:
	@GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING=$(GOOSE_DBSTRING) GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) goose up

migrate-down:
	@GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING=$(GOOSE_DBSTRING) GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) goose down

migrate-status:
	@GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING=$(GOOSE_DBSTRING) GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) goose status

seed:
	@cd backend && go run ./cmd/seed --scenario=$(SCENARIO)

test-backend:
	@cd backend && go test ./...

test-integration:
	@cd backend && go test -tags=integration ./...

prod-up:
	@docker compose -f docker-compose.prod.yml --env-file .env up -d

prod-down:
	@docker compose -f docker-compose.prod.yml down

prod-logs:
	@docker compose -f docker-compose.prod.yml logs -f

prod-pull:
	@docker compose -f docker-compose.prod.yml pull
