#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/compose.yaml"
PROJECT_NAME="${TEST_COMPOSE_PROJECT:-rota-integration-$(date +%s)-$$}"
# Refuse to reuse any existing project: tests truncate fixtures and cleanup
# removes this project's volume. A test must never target an existing stack.
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT_NAME")" || -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT_NAME")" ]]; then
  echo "Refusing to reuse existing Compose project: $PROJECT_NAME" >&2
  exit 1
fi
TEMP_DIR="$(mktemp -d)"
touch "$TEMP_DIR/env"
compose=(docker compose --env-file "$TEMP_DIR/env" -p "$PROJECT_NAME" -f "$COMPOSE_FILE")
# Whole-file Compose interpolation requires these even when only postgres and
# migrate start. Do not read the user's application .env or inherit its DB host.
key() { openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n'; }
export PASSWORD_RESET_TOKEN_KEY="$(key)" INVITATION_TOKEN_KEY="$(key)" EMAIL_SETTINGS_ENCRYPTION_KEY="$(key)"
export POSTGRES_HOST=postgres POSTGRES_PORT=5432 POSTGRES_SSLMODE=disable

DB_USER="${POSTGRES_USER:-temvia}"
DB_PASSWORD="${POSTGRES_PASSWORD:-pa55word}"
DB_NAME="${POSTGRES_DB:-temvia}"
DB_HOST_PORT="${POSTGRES_HOST_PORT:-15432}"

cleanup() {
  if [[ "${KEEP_TEST_DB:-}" != "1" ]]; then
    "${compose[@]}" down -v --remove-orphans >/dev/null
  fi
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

export POSTGRES_USER="$DB_USER"
export POSTGRES_PASSWORD="$DB_PASSWORD"
export POSTGRES_DB="$DB_NAME"
export POSTGRES_HOST_PORT="$DB_HOST_PORT"

"${compose[@]}" up -d postgres >/dev/null

for _ in {1..60}; do
  if "${compose[@]}" exec -T postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! "${compose[@]}" exec -T postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
  "${compose[@]}" logs postgres >&2
  echo "integration test Postgres did not become ready." >&2
  exit 1
fi

endpoint="$("${compose[@]}" port postgres 5432)"
port="${endpoint##*:}"
export TEST_POSTGRES_DSN="postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${port}/${DB_NAME}?sslmode=disable"

echo "Applying API migrations with the isolated migration image" >&2
"${compose[@]}" --profile tools run --rm --build migrate

echo "Running API integration tests against Postgres on 127.0.0.1:${port}" >&2
cd "$ROOT_DIR/api"
if [[ "$#" -eq 0 ]]; then
  set -- ./...
fi

# Integration packages share one disposable database and several package-level
# fixtures intentionally reset common auth/Rota state. Serialize package tests
# so one package cannot truncate another package's fixture while it is running.
go test -p 1 -tags=integration "$@"
