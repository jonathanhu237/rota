#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.test.yml"
PROJECT_NAME="${TEST_COMPOSE_PROJECT:-rota-integration-test}"

DB_USER="${POSTGRES_USER:-rota}"
DB_PASSWORD="${POSTGRES_PASSWORD:-pa55word}"
DB_NAME="${POSTGRES_DB:-rota}"

cleanup() {
  if [[ "${KEEP_TEST_DB:-}" != "1" ]]; then
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null
  fi
}
trap cleanup EXIT

if ! command -v goose >/dev/null 2>&1; then
  echo "goose is required to run integration tests because migrations must be applied first." >&2
  exit 127
fi

export POSTGRES_USER="$DB_USER"
export POSTGRES_PASSWORD="$DB_PASSWORD"
export POSTGRES_DB="$DB_NAME"

docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --remove-orphans postgres >/dev/null

for _ in {1..60}; do
  if docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" exec -T postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" exec -T postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" logs postgres >&2
  echo "integration test Postgres did not become ready." >&2
  exit 1
fi

endpoint="$(docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" port postgres 5432)"
port="${endpoint##*:}"
export DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${port}/${DB_NAME}?sslmode=disable"

echo "Running integration tests against Postgres on 127.0.0.1:${port}" >&2

GOOSE_DRIVER=postgres \
GOOSE_DBSTRING="$DATABASE_URL" \
GOOSE_MIGRATION_DIR="$ROOT_DIR/migrations" \
  goose up

cd "$ROOT_DIR/backend"
if [[ "$#" -eq 0 ]]; then
  set -- ./...
fi

go test -tags=integration "$@"
