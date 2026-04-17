# Rota

A shift scheduling system for university work-study program.

## Testing

- Backend unit tests: `make test-backend`
- Backend integration tests: `make test-integration`
- Integration tests expect Postgres to be reachable with the configured `POSTGRES_*` environment variables and with the migrations already applied.
