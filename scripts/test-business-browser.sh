#!/usr/bin/env bash
# Run only against a new, isolated Compose project. Never uses the user's .env.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
work="$(mktemp -d)"
project="rota-business-$(date +%s)-$$"
export ADMIN_PORT="${E2E_ADMIN_PORT:-27773}" API_PORT="${E2E_API_PORT:-27780}"
export POSTGRES_HOST_PORT="${E2E_POSTGRES_PORT:-27732}" MAILPIT_UI_PORT="${E2E_MAILPIT_PORT:-27725}"
export APP_PUBLIC_URL="http://127.0.0.1:$ADMIN_PORT"
# Explicit values prevent ambient development configuration pointing at real data.
export POSTGRES_HOST=postgres POSTGRES_PORT=5432 POSTGRES_DB=rota POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_SSLMODE=disable
export APP_ENV=development HTTP_ADDR=0.0.0.0:8080
key() { openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n'; }
export PASSWORD_RESET_TOKEN_KEY="$(key)"
export INVITATION_TOKEN_KEY="$(key)"
export EMAIL_SETTINGS_ENCRYPTION_KEY="$(key)"
export EMAIL_CHANGE_CODE_KEY="$(key)"
touch "$work/env"
compose=(docker compose --env-file "$work/env" -p "$project" -f "$ROOT_DIR/compose.yaml")
cleanup() {
  "${compose[@]}" --profile development --profile tools down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
"${compose[@]}" build api admin migrate
"${compose[@]}" up -d --wait postgres
"${compose[@]}" --profile tools run --rm migrate up
"${compose[@]}" --profile development up -d api admin mailpit
for attempt in $(seq 1 60); do
  if curl -fsS "$APP_PUBLIC_URL/api/public/system-identity" >/dev/null 2>&1; then break; fi
  if [[ "$attempt" == 60 ]]; then echo 'API startup timeout' >&2; exit 1; fi
  sleep 1
done
"${compose[@]}" logs --no-color api > "$work/startup.log"
STARTUP_LOG="$work/startup.log" node --input-type=module <<'JS'
import { readFileSync } from 'node:fs';
const match = readFileSync(process.env.STARTUP_LOG, 'utf8').match(/\/setup#token=([^\s]+)/);
if (!match) throw new Error('Missing first-run setup authority');
const response = await fetch(`${process.env.APP_PUBLIC_URL}/api/setup`, {
  method: 'POST', headers: { 'Content-Type': 'application/json', Origin: process.env.APP_PUBLIC_URL },
  body: JSON.stringify({ token: match[1], name: 'Browser Admin', email: 'admin@example.com', password: 'Admin1!x', locale: 'en' }),
});
if (!response.ok) throw new Error(`Setup failed (${response.status})`);
JS
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U rota -d rota < "$ROOT_DIR/scripts/fixtures/business-browser.sql"
cd admin
E2E_ROTA_BUSINESS=1 PLAYWRIGHT_BASE_URL="$APP_PUBLIC_URL" E2E_MAILPIT_API_URL="http://127.0.0.1:$MAILPIT_UI_PORT" \
  pnpm exec playwright test e2e/rota-business.spec.ts --project=chromium --workers=1 --retries=0
