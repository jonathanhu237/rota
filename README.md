# Rota

A shift scheduling system for departments.

## Prerequisites

- Docker and Docker Compose
- Go 1.26.1
- Node 22 with `pnpm`
- `goose` for local migration commands

## Development Setup

1. Copy `.env.example` to `.env`.
2. Set local values for Postgres, Redis, bootstrap admin, and any SMTP settings you want to test.
3. Start Postgres and Redis:
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

### Seeding Dev Data

`make seed` wipes the configured local Postgres data tables with `TRUNCATE ... RESTART IDENTITY CASCADE`, then inserts a known-good development dataset. It prints the target database before wiping, refuses to run when `APP_ENV=production`, and uses `pa55word` for all seeded user passwords unless the bootstrap admin password is set in `.env`.

```bash
make seed
make seed SCENARIO=full
make seed SCENARIO=stress
```

Scenarios:

- `basic`: bootstrap admin, 5 employees, 3 positions, and one empty template.
- `full`: 8 employees, 4 positions, a populated template, one effective `ASSIGNING` publication, and availability submissions ready for auto-assign.
- `stress`: 50 employees, 8 positions, dense template data, one `ACTIVE` publication with assignments and pending shift-change requests, plus ended historical fixture data. The database permits only one non-`ENDED` publication at a time.

## Testing

- Backend unit tests: `make test-backend`
- Backend integration tests: `make test-integration`
- Frontend tests: `cd frontend && pnpm test`
- Frontend build: `cd frontend && pnpm build`

Integration tests expect Postgres to be reachable with the configured `POSTGRES_*` environment variables and with the migrations already applied.

## Production Deployment

1. Copy `.env.example` to `.env`.
2. Fill in the production values, especially `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `BOOTSTRAP_ADMIN_PASSWORD`, `SMTP_*`, `APP_BASE_URL`, and `CADDY_SITE_ADDRESS`.
3. Bring up the full stack:
   ```bash
   make prod-up
   ```
4. Open the URL served by Caddy.
5. Log in with the bootstrap admin user from `.env`.

The production stack includes Postgres, Redis, a one-shot migration runner, the Go backend, and Caddy serving the SPA plus reverse proxying `/api/*` to the backend.

### HTTPS

- Set `CADDY_SITE_ADDRESS` to a real domain such as `rota.example.com` to let Caddy obtain and renew certificates automatically.
- For local Docker testing, keep `CADDY_SITE_ADDRESS=http://localhost` so Caddy serves plain HTTP on port 80.
- `APP_BASE_URL` must match the public URL you expect users to open from invitation and password-reset emails.

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
| `REDIS_HOST` | `localhost` | Redis host for local development. Production Compose overrides this to `redis`. |
| `REDIS_PORT` | `6379` | Redis port. |
| `REDIS_PASSWORD` | empty | Redis password. Set a real value before deploy. |
| `REDIS_DB` | `0` | Redis database index. |
| `SESSION_EXPIRES_HOURS` | `336` | Session TTL in hours. |
| `EMAIL_MODE` | `log` | Selects the logger or SMTP emailer. |
| `SMTP_HOST` | empty | SMTP server host. Must be set for production email delivery. |
| `SMTP_PORT` | `587` | SMTP server port. |
| `SMTP_USER` | empty | SMTP username. Do not commit real credentials. |
| `SMTP_PASSWORD` | empty | SMTP password. Do not commit real credentials. |
| `SMTP_FROM` | `Rota <noreply@example.com>` | Default sender address. |
| `SMTP_TLS_MODE` | `starttls` | SMTP TLS mode: `starttls`, `implicit`, or `none`. |
| `CADDY_SITE_ADDRESS` | `http://localhost` | Public site address for Caddy. Use a real domain in production. |
| `APP_BASE_URL` | `http://localhost:5173` | Base URL embedded in invitation and reset emails. |
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
