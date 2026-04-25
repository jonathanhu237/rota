## 1. Skeleton

- [ ] 1.1 Create `backend/cmd/seed/main.go` with: env loading via `config.Load()`, production guard (panic on `cfg.AppEnv == "production"`), `--scenario` flag parsing (default `basic`, accepted values `basic|full|stress`), DB open. Verify: `cd backend && go build ./cmd/seed`.
- [ ] 1.2 Create `backend/cmd/seed/internal/wipe.go` with `func WipeAllData(ctx context.Context, tx *sql.Tx) error`. Implementation: a single `TRUNCATE TABLE ... RESTART IDENTITY CASCADE` covering `audit_logs, shift_change_requests, assignments, availability_submissions, user_setup_tokens, user_positions, template_slot_positions, template_slots, publications, templates, positions, users`. Verify: `cd backend && go build ./...`.
- [ ] 1.3 Create `backend/cmd/seed/internal/users.go` with `func InsertUser(tx, email, name, password, isAdmin) (int64, error)`. Implementation: hash password via `bcrypt.GenerateFromPassword(... bcrypt.DefaultCost)`, INSERT user row with `status='active'`, return id. Verify: same.
- [ ] 1.4 Add a top-of-binary stdout banner: `WIPING database <db>@<host>:<port> ...` so it's visible what's about to be destroyed. Add 1-second sleep when `isatty(stdout)` to be a faint safety pause; skip if non-TTY (CI). Verify: `cd backend && go run ./cmd/seed --scenario=basic` (against a local DB) prints the banner.

## 2. Scenario: basic

- [ ] 2.1 Create `backend/cmd/seed/scenarios/basic.go` with `func Run(ctx, tx) error`. Implementation:
  - Reseed bootstrap admin (email/name/password from `BOOTSTRAP_ADMIN_*` env, `is_admin=true`).
  - Insert 5 employees (`employee1..5@example.com`, name `Employee N`, password `"pa55word"`, `is_admin=false`).
  - Insert 3 positions: `"Position A"`, `"Position B"`, `"Position C"`.
  - Insert 1 template: `"Default Rota"`, no slots.
  - No publications, submissions, or assignments.
  Verify: `make seed` → opens psql, `SELECT count(*) FROM users` shows 6, `SELECT count(*) FROM positions` shows 3, `SELECT count(*) FROM templates` shows 1. Login as `employee1@example.com` / `pa55word` succeeds.

## 3. Scenario: full

- [ ] 3.1 Create `backend/cmd/seed/scenarios/full.go` with `func Run(ctx, tx) error`. Implementation:
  - Insert bootstrap admin + 8 employees (`employee1..8@example.com`).
  - Insert 4 positions: A/B/C/D.
  - Each employee qualified for 2 positions (round-robin).
  - 1 template with ~10 slots spanning Mon-Fri × 2-3 daytime windows + 1 Mon-Sun evening slot.
  - Slot-position composition: `{A=1, B=1}` on most slots; `{C=1, D=1}` on the evening slot.
  - 1 publication: `state='DRAFT'`, `submission_start_at=NOW()-INTERVAL '14 days'`, `submission_end_at=NOW()-INTERVAL '7 days'`, `planned_active_from=NOW()+INTERVAL '7 days'`. Effective state resolves to `ASSIGNING`.
  - Availability submissions: each employee submits ~60% of the qualified `(slot, position)` pairs in this publication. Use a deterministic pseudo-random selection (e.g., `(employee_id + slot_id + position_id) % 5 < 3`).
  - 0 assignments.
  Verify: `make seed SCENARIO=full` succeeds. Counts: 9 users, 4 positions, 10 slots, ~16 slot_positions, 1 publication, ~50-70 submissions, 0 assignments. As admin, `POST /publications/1/auto-assign` runs and returns a non-empty assignment set.

## 4. Scenario: stress

- [ ] 4.1 Confirm whether `publications` has a partial unique index on `state != 'ENDED'`. Read the migration file (`migrations/00009_*.sql` or similar) to verify. If the index exists, design `stress` so that exactly one publication is non-ENDED at any moment. If it doesn't exist, the seed can be more aggressive. Document the finding in this task. Verify: `psql -c "\d publications"` reveals the indexes.
- [ ] 4.2 Create `backend/cmd/seed/scenarios/stress.go` with `func Run(ctx, tx) error`. Implementation, conditioned on §4.1's finding:
  - Insert bootstrap admin + 50 employees.
  - 8 positions.
  - Each employee qualified for 2-3 positions.
  - 1 template, ~20 slots (Mon-Fri × 4 daytime slots × 3 positions + Mon-Sun × 1 evening × 2 positions).
  - 4 publications: 2 ENDED (historical), 1 ACTIVE (current week), 1 ASSIGNING (next week). Adjust state values to satisfy the D2 invariant once §4.1 is resolved.
  - For the ACTIVE publication: full assignment coverage (~40 rows).
  - For the ASSIGNING publication: ~40% submissions, 0 assignments.
  - 3 pending shift-change requests (1 swap, 1 give_direct, 1 give_pool) on the ACTIVE publication.
  Verify: `make seed SCENARIO=stress` succeeds. Counts: 51 users, 8 positions, ~20 slots, ~80 slot_positions, 4 publications, hundreds of submissions, ~40 assignments, 3 shift_change_requests. UI loaded against this DB feels like a "real" deployment.

## 5. Makefile target

- [ ] 5.1 Add `seed:` target to root `Makefile`:
  ```
  SCENARIO ?= basic
  
  seed:
  	cd backend && go run ./cmd/seed --scenario=$(SCENARIO)
  ```
  Verify: `make seed`, `make seed SCENARIO=full`, `make seed SCENARIO=stress` all run end-to-end.

## 6. Production guard test

- [ ] 6.1 Add a unit test in `backend/cmd/seed/main_test.go` that loads a fake `cfg.AppEnv = "production"` and asserts the binary's main function (extracted to a testable `func run(cfg) error`) returns an error. Verify: `cd backend && go test ./cmd/seed -count=1`.

## 7. Documentation

- [ ] 7.1 Add a "Seeding dev data" subsection to the project's README (or `AGENTS.md` Commands section) covering: what `make seed` does, scenario differences, the wipe-and-reseed contract, the production guard, and the password convention (`pa55word` for all seeded users). Verify: render the README locally and re-read for clarity.

## 8. Final verification

- [ ] 8.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [ ] 8.2 Smoke test:
  - (a) `make migrate-down && make migrate-up && make seed`. Login as `admin@example.com` / `pa55word`. Verify positions, template, employees visible in admin UI (or via `GET /api/users`, `/api/positions`, `/api/templates`).
  - (b) `make migrate-down && make migrate-up && make seed SCENARIO=full`. Login as admin, `POST /api/publications/1/auto-assign`, verify assignments materialize.
  - (c) `make migrate-down && make migrate-up && make seed SCENARIO=stress`. Open the assignment-board UI, observe ACTIVE publication with full coverage.
  - (d) Set `AppEnv=production` in `.env` (back it up first), run `make seed`. Verify it refuses with a clear message and exits non-zero. Restore `.env`.
- [ ] 8.3 Confirm the existing CI (`backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`) is unaffected by this change. CI does not run `make seed` and should not need to.
