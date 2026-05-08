# Rota

A shift scheduling system for departments.

## Prerequisites

- Docker and Docker Compose
- Go 1.26.1
- Node 22 with `pnpm`
- `goose` for local migration commands

## Development Setup

1. Copy `.env.example` to `.env`.
2. Set local values for Postgres, bootstrap admin, and any SMTP settings you want to test.
3. Start Postgres:
   ```bash
   docker compose up -d
   ```
4. Apply migrations:
   ```bash
   make migrate-up
   ```
5. Start the backend:
   ```bash
   make run-backend
   ```
6. Start the frontend in another shell:
   ```bash
   make run-frontend
   ```

The Vite dev server uses `/api/*` and proxies those requests to `http://localhost:8080`, which matches the production Caddy routing shape.

## Architecture Decisions

- [ADR 0001: Prefer Postgres-backed jobs before introducing a message queue](docs/adr/0001-postgres-backed-jobs-before-message-queue.md)
- [ADR 0002: Cache reads per read model, not through a generic repository layer](docs/adr/0002-cache-reads-per-read-model.md)
- [ADR 0003: Defer Redis until there is a measured runtime need](docs/adr/0003-defer-redis-runtime-dependency.md)

### Seeding Dev Data

`make seed` wipes the configured local Postgres data tables with `TRUNCATE ... RESTART IDENTITY CASCADE`, then inserts a known-good development dataset. It prints the target database before wiping, refuses to run when `APP_ENV=production`, and uses `pa55word` for all seeded user passwords unless the bootstrap admin password is set in `.env`.

```bash
make seed
make seed SCENARIO=full
make seed SCENARIO=stress
```

Scenarios:

- `basic`: bootstrap admin, 5 employees, 3 positions, and one empty template.
- `full`: 8 employees, 4 roles, a populated template, one effective `ASSIGNING` publication with an 8-week planned active window, and slot-level availability submissions ready for auto-assign.
- `stress`: 50 employees, 8 positions, dense template data, one `ACTIVE` publication with assignments and pending occurrence-level shift-change requests, plus ended historical fixture data with varied planned active windows. The database permits only one non-`ENDED` publication at a time.

### Leave Workflow

Employees use `/leaves` as the leave workbench. The page opens on the pending leave pool, shows public requests that anyone can help cover, shows direct requests only to the requester, the specified colleague, and admins, and keeps completed/cancelled/failed rows available through status filters. Each row links to `/leaves/:id`, displays the requester, shift time, position, category, counterpart or substitute when available, urgency, and the backend-provided actions the viewer may take.

Employees create leave requests from `/leaves/new` by selecting a date range, choosing an upcoming assigned occurrence, and submitting either public coverage or direct coverage with a leave category and optional reason. Direct coverage candidates are limited to active qualified colleagues for that occurrence; public coverage is added to the pool without broadcasting email to every possible helper. A pending leave does not transfer responsibility: the requester remains assigned until a qualified colleague claims the public request or the specified colleague approves the direct request. Successful coverage transfers only that occurrence; uncovered requests fail after the occurrence start.

Admins can view leave rows but do not approve, reject, claim, or cancel on behalf of employees in this workflow. Admin publication-level compatibility remains available through `GET /api/publications/{id}/leaves`.

## Testing

- Backend unit tests: `make test-backend`
- Backend integration tests: `make test-integration`
- Frontend tests: `cd frontend && pnpm test`
- Frontend build: `cd frontend && pnpm build`

`make test-integration` starts an isolated Docker Postgres instance, publishes it on a random local port so it does not conflict with an existing `5432`, applies migrations, runs the Go integration tests, and removes the test database afterward. Pass package or test filters through `TEST_ARGS`, for example:

```bash
make test-integration TEST_ARGS="./internal/repository -run TestPublicationRepositoryIntegration/ReplaceAdminAvailabilitySubmissions -v"
```

Use `KEEP_TEST_DB=1 make test-integration` to leave the temporary database running for debugging.

## Production Deployment

1. Copy `.env.example` to `.env`.
2. Fill in the production values, especially `POSTGRES_PASSWORD`, `BOOTSTRAP_ADMIN_PASSWORD`, `SMTP_*`, `APP_BASE_URL`, `CADDY_SITE_ADDRESS`, and the Caddy TLS mode.
3. Bring up the full stack:
   ```bash
   make prod-up
   ```
4. Open the URL served by Caddy.
5. Log in with the bootstrap admin user from `.env`.

The production stack includes Postgres, a one-shot migration runner, the Go backend, and Caddy serving the SPA plus reverse proxying `/api/*` to the backend.

### HTTPS

- Set `CADDY_SITE_ADDRESS` to a real domain such as `rota.example.com` to let Caddy obtain and renew certificates automatically.
- For local Docker testing, keep `CADDY_SITE_ADDRESS=http://localhost` so Caddy serves plain HTTP on port 80.
- `APP_BASE_URL` must match the public URL you expect users to open from invitation and password-reset emails.

Use `CADDY_TLS_MODE=auto` for a public domain where Caddy can complete ACME validation through public HTTP/TLS challenges:

```env
CADDY_SITE_ADDRESS=rota.example.com
APP_BASE_URL=https://rota.example.com
CADDY_TLS_MODE=auto
```

Use `CADDY_TLS_MODE=manual` when the service is only reachable on an intranet, but you already have a publicly trusted certificate for the hostname, for example one obtained through DNS-01 validation. Put the certificate files on the server under `./certs/` (ignored by git), and keep the in-container paths in `.env`:

```env
CADDY_SITE_ADDRESS=https://rota.example.com
APP_BASE_URL=https://rota.example.com
CADDY_TLS_MODE=manual
CADDY_TLS_CERT_FILE=/certs/fullchain.pem
CADDY_TLS_KEY_FILE=/certs/privkey.pem
```

The production Compose stack mounts `./certs` into the Caddy container read-only as `/certs`. Certificate issuance and renewal are intentionally handled outside this project, for example with `acme.sh` or `certbot` using DNS-01.

### Useful Production Commands

```bash
make prod-up
make prod-down
make prod-logs
make prod-pull
```

### Environment Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_ENV` | `development` | Runtime environment name. `make seed` refuses to run when this is `production`. |
| `SERVER_PORT` | `8080` | Backend listen port inside the container. |
| `POSTGRES_HOST` | `localhost` | Database host for local development. Production Compose overrides this to `postgres`. |
| `POSTGRES_PORT` | `5432` | Database port. |
| `POSTGRES_USER` | `rota` | Database user. |
| `POSTGRES_PASSWORD` | empty | Database password. Set a real value before deploy. |
| `POSTGRES_DB` | `rota` | Database name. |
| `SESSION_EXPIRES_HOURS` | `336` | Session TTL in hours. |
| `EMAIL_MODE` | `log` | Selects the logger or SMTP emailer. |
| `SMTP_HOST` | empty | SMTP server host. Must be set for production email delivery. |
| `SMTP_PORT` | `587` | SMTP server port. |
| `SMTP_USER` | empty | SMTP username. Do not commit real credentials. |
| `SMTP_PASSWORD` | empty | SMTP password. Do not commit real credentials. |
| `SMTP_FROM` | `Rota <noreply@example.com>` | Default sender address. |
| `SMTP_TLS_MODE` | `starttls` | SMTP TLS mode: `starttls`, `implicit`, or `none`. |
| `EMAIL_SEND_TIMEOUT` | `30s` | Per-message outbox send timeout. |
| `CADDY_SITE_ADDRESS` | `http://localhost` | Public site address for Caddy. Use a real domain in production. |
| `APP_BASE_URL` | `http://localhost:5173` | Base URL embedded in invitation and reset emails. |
| `CADDY_TLS_MODE` | `auto` | Selects the Caddy config: `auto` for Caddy automatic HTTPS, `manual` for loading files from `CADDY_TLS_CERT_FILE` and `CADDY_TLS_KEY_FILE`. |
| `CADDY_TLS_CERT_FILE` | `/certs/fullchain.pem` | Certificate chain path inside the Caddy container when `CADDY_TLS_MODE=manual`. |
| `CADDY_TLS_KEY_FILE` | `/certs/privkey.pem` | Private key path inside the Caddy container when `CADDY_TLS_MODE=manual`. |
| `INVITATION_TOKEN_TTL` | `72h` | Invitation link lifetime. |
| `PASSWORD_RESET_TOKEN_TTL` | `1h` | Password reset link lifetime. |
| `BOOTSTRAP_ADMIN_EMAIL` | `admin@example.com` | Initial admin email when the database is empty. |
| `BOOTSTRAP_ADMIN_PASSWORD` | empty | Initial admin password. Set before deploy and do not commit the real value. |
| `BOOTSTRAP_ADMIN_NAME` | `Administrator` | Initial admin display name. |

## Backup

Create a PostgreSQL backup from the production stack with:

```bash
docker compose -f docker-compose.prod.yml exec postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > rota.sql
```
