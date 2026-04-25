## Why

Manual smoke testing today requires ~10 minutes of curl setup before any test logic runs: log in as bootstrap admin, create positions, build a template with slots and slot-positions, invite users (and intercept the invitation tokens to set passwords), grant qualifications, create a publication, advance state, optionally seed availability submissions. We have hit this friction every time we smoke-tested a change in this session — twice for the slot refactor, once for tighten-registration-flow, once for tighten-scheduling-edges. Each round the setup is the same and each round it costs more in elapsed time than the actual test.

The friction makes routine manual testing prohibitively expensive. The smoke test most likely to catch a real regression — "click through the affected flow as an admin and as an employee" — gets skipped or rushed because the data setup dominates. Two recent bugs (C1 silent-upsert in slot refactor, the SCHEDULING_RETRYABLE / GIST insight in tighten-scheduling-edges) were caught precisely because we forced ourselves through the painful setup once. Both could have been caught earlier had the setup been a one-liner.

A `make seed` command that produces a known-good database state in seconds removes the friction. With seed in place, every future change's smoke test becomes "make migrate-down && make migrate-up && make seed && curl ... ". The marginal cost of running smoke drops to under a minute, so we actually run it.

## What Changes

- New Go binary at `backend/cmd/seed/main.go` that connects to the same Postgres the server uses and writes test data.
- Three scenarios selectable by `--scenario` flag:
  - `basic`: minimum data to navigate the admin UI without empty-state errors (5 active users, 3 positions, 1 empty template).
  - `full`: enough state to immediately exercise auto-assign and shift-change flows (basic + populated template + ASSIGNING-state publication + availability submissions; admin runs auto-assign to fill assignments).
  - `stress`: large-volume data for UI density and rough performance feel (50 users, 8 positions, populated template, one ACTIVE publication, and ENDED historical/fixture publications).
- Implementation goes through **direct SQL** (not the service layer): the Go binary computes bcrypt password hashes itself, runs raw `INSERT` statements wrapped in a transaction per scenario, and skips audit-log emission on purpose — seed data should not pollute the audit trail.
- Idempotency by **wipe-and-reseed**: each `make seed` call begins by `TRUNCATE`-ing the data tables (everything except `goose_db_version`) inside a transaction, then inserting fresh rows. The bootstrap admin row is recreated to match `BOOTSTRAP_ADMIN_*` env vars.
- Production guard: the binary refuses to run when `cfg.AppEnv == "production"` (panics on startup with a clear message). No flag overrides this — production seeding is not a use case.
- Makefile target `make seed` (default scenario `basic`) and `make seed SCENARIO=full|stress`. Both `make seed` and direct `go run ./backend/cmd/seed` are supported and behave identically.

## Capabilities

### New Capabilities

- `dev-tooling`: introduces a `make seed` command (and underlying Go binary `cmd/seed`) as the project's contract for "seed a known-good database state for local development." Captures the production guard, the three scenarios, and the wipe-and-reseed semantics so future changes can extend or modify the contract intentionally.

### Modified Capabilities

None. The user-facing application contract (HTTP endpoints, auth flows, scheduling behavior) is unchanged.

## Non-goals

- **Running seed in production.** Explicitly rejected at startup; not behind a flag, not behind a feature toggle.
- **Going through the service layer.** Direct SQL is the right choice for seed: it skips audit log pollution, doesn't trigger invitation emails, doesn't run state-machine transitions, and runs in seconds. The cost is mirroring the schema in seed code (acceptable; schema changes already touch a handful of files).
- **Soft-merge / preserve-existing-data semantics.** Each `make seed` invocation wipes and reseeds. If a developer has hand-built data they want to preserve, they should not run `make seed`.
- **A separate "test-fixture" scenario for integration tests.** Integration tests already have inline seed helpers and should keep them; their needs and lifetime differ from `make seed`'s.
- **Generating realistic Chinese names / emails / Faker-style data.** Seeded users use placeholder names (`Employee 1`, `Employee 2`, ...) and `employee1@example.com`-style emails. No real names, no Faker dependency.
- **A "wipe only" scenario.** `make migrate-down && make migrate-up` already does this; an `empty` scenario would be redundant.

## Impact

- **New backend binary**:
  - `backend/cmd/seed/main.go`: env loading (reuses `config.Config`), production guard, `--scenario` flag dispatch.
  - `backend/cmd/seed/scenarios/basic.go`, `full.go`, `stress.go`: per-scenario SQL.
  - `backend/cmd/seed/internal/wipe.go`: TRUNCATE helper covering every data table (excluding `goose_db_version`).
  - `backend/cmd/seed/internal/users.go`: bcrypt-hash + INSERT helpers shared across scenarios.
- **Makefile**:
  - New target `seed`. Reads `SCENARIO` make-variable (defaults to `basic`); calls `cd backend && go run ./cmd/seed --scenario=$(SCENARIO)`.
- **README / dev docs**: a short "Seeding dev data" section under the existing local-dev section. Document `make seed` / `make seed SCENARIO=full` / `make seed SCENARIO=stress`, the production guard, and the wipe-and-reseed semantics.
- **No frontend changes.**
- **No database migration.** Seed only inserts into existing tables.
- **No new dependencies**: bcrypt is already imported transitively via `golang.org/x/crypto/bcrypt`; SQL drivers and config loader are already in use.

## Risks / safeguards

- A developer running `make seed` against a database with valuable hand-built state loses that state. **Mitigation**: the wipe step prints a clear "WIPING database <db_name> on <host>:<port>" line before truncating, and the production guard prevents the worst case. Beyond that, this is the documented contract — `make seed` means "I want a fresh state."
- Schema drift between seed and migrations. **Mitigation**: seed runs the same DB the server uses; if the schema changes and seed's INSERT references a removed/renamed column, the seed binary fails to compile or fails at runtime. The CI's existing `migrations-roundtrip` job does not run seed, but seed is run in local dev frequently, so drift surfaces quickly.
