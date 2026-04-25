## Context

We have shipped four behavior changes in this codebase across this session (`admin-shift-adjustments`, `refactor-to-slot-position-model`, `tighten-registration-flow`, `tighten-scheduling-edges`), and each one ended with manual smoke testing that consumed substantially more setup time than test time. The setup is identical every round — log in as bootstrap admin, manually curl-create positions / template / slots / users / publication / submissions — and contributes nothing to validation. Removing that friction is the goal of this change.

`seed-dev-data` is a development convenience tool, not a runtime capability. It writes a known-good state to the configured Postgres database in seconds, so smoke testing reduces to "wipe → seed → click". The tool is dev-only by design: it refuses to run against `AppEnv == "production"`.

## Goals / Non-Goals

**Goals:**

- A single command (`make seed`) that resets the database to a known scenario in seconds.
- Three scenarios — `basic`, `full`, `stress` — covering the common testing modes.
- All seeded user accounts have working credentials (real bcrypt hashes; password is `pa55word`).
- Seed leaves the database in a state the application considers valid: assignments reference existing slots, slots reference existing templates, etc. No dangling FKs, no UNIQUE / CHECK / GIST violations.
- Production is hard-blocked at process startup.

**Non-Goals:**

- Going through the service layer (rejected — see Decisions).
- Preserving hand-built local data (wipe-and-reseed is the contract).
- Running in production.
- Replacing integration-test fixtures.
- Generating realistic-looking names or using Faker-style libraries.

## Decisions

### Direct SQL, not service layer

A natural first instinct is "go through the service layer so seeded data is realistic." Rejected. For seed specifically, direct SQL is better because:

1. **No audit pollution.** Service-layer methods emit `audit.Record` events. Seeding 50 users would write 50 `user.create` audit rows. That's noise that contaminates the audit log and makes future audit-related queries harder to read. Direct SQL writes only the rows you ask for.
2. **No SMTP attempts.** `userService.CreateUser` calls `sendInvitation` after the tx commits. Even with `EMAIL_MODE=log`, the call goes through the emailer interface; setting up a mute mode would mean a special seed-only flag on the service. Direct SQL skips this entirely.
3. **No state-machine traversal.** To get a publication to `ASSIGNING` state via the service layer, we would have to create it in `DRAFT`, transition through the time-based effective state resolution, possibly run additional service calls. Direct SQL: set `submission_start_at` to the past and `submission_end_at` to the past — done.
4. **Speed.** A bulk `INSERT INTO users ... VALUES (...), (...), (...)` writing 50 users in one round trip is far faster than 50 sequential `userService.CreateUser` calls.
5. **Bcrypt is one Go import away.** The single argument for service layer was "real bcrypt hashes." But the seed binary is in Go and can call `bcrypt.GenerateFromPassword` directly without touching the service layer.

**Trade-off accepted**: seed code mirrors the schema. If a column is added to `users`, seed's INSERT statement may need an update. This is a maintenance task that already exists for any seed approach (the alternative being for seed to magically discover schema changes), and it surfaces fast — the binary fails to compile or fails at runtime on the next dev run.

**Trade-off accepted**: seed can technically write rows that the application considers semi-invalid (e.g., a `users` row with `status='active'` but `password_hash=''`). The seed code is short and easy to audit; we treat it as the developer's responsibility to keep seed data consistent with what the application expects.

### Wipe-and-reseed idempotency

Every `make seed` call begins with a `TRUNCATE` of every data table (excluding `goose_db_version`), inside a transaction, before any INSERT. This is the simplest semantically clear behavior. Two alternatives were considered and rejected:

- **`ON CONFLICT DO UPDATE`**: complex, reads the old row, writes the new, hard to reason about when seed data and developer's hand-built data conflict. Rejected.
- **Refuse to run when data exists**: forces developer to `make migrate-down && make migrate-up && make seed`, which is exactly the friction we're trying to remove. Rejected.

Implementation:

```sql
BEGIN;
TRUNCATE TABLE
  audit_logs,
  shift_change_requests,
  assignments,
  availability_submissions,
  user_setup_tokens,
  user_positions,
  template_slot_positions,
  template_slots,
  publications,
  templates,
  positions,
  users
RESTART IDENTITY CASCADE;
-- ... INSERTs ...
COMMIT;
```

`RESTART IDENTITY` resets `BIGSERIAL` sequences so that every seed run starts from id 1. `CASCADE` is defensive — the table list above is in delete-safe order, but `CASCADE` makes the order forgiving.

### Production guard

In `main.go` before any DB work:

```go
cfg, err := config.Load()
if err != nil { return err }
if cfg.AppEnv == "production" {
    return fmt.Errorf("seed: refusing to run with AppEnv=production")
}
```

No flag override. If a developer wants to seed a production-mirror staging environment, they set `AppEnv=staging` (or similar) — but seed should never touch a real production database. The cost of being able to bypass this is a category of disasters worse than any cost of being unable to bypass it; we err on the side of an absolute block.

### Three scenarios, no "empty"

`basic`, `full`, and `stress` are documented below. We intentionally omit an `empty` scenario:

- `make migrate-down && make migrate-up` produces an empty database in a single command already.
- An `empty` scenario would just call the wipe step and return, duplicating that behavior under a different name.

#### Scenario: basic

Minimum to navigate the admin UI without empty-state errors. ~10 seconds to seed.

```
users:
  - admin@example.com (bootstrap admin, recreated)
  - employee1@example.com .. employee5@example.com (5 active users, password "pa55word")
positions:
  - "Position A", "Position B", "Position C" (placeholder names, anonymized)
template: 1, no slots
publications: 0
submissions: 0
assignments: 0
shift_change_requests: 0
```

#### Scenario: full

Enough to run `POST /publications/{id}/auto-assign` immediately and see results. ~15 seconds.

```
users: bootstrap admin + 8 employees (password "pa55word")
positions: 4 ("Position A".."D")
user_positions: each employee qualified for 2 positions
template: 1, with slots:
  Mon 09:00-12:00 → {A=1, B=1}
  Mon 14:00-17:00 → {A=1, B=1}
  Tue 09:00-12:00 → {A=1, B=1}
  Tue 14:00-17:00 → {A=1, C=1}
  Wed 09:00-12:00 → {A=1, B=1}
  Wed 19:00-21:00 → {C=1, D=1}
  ...
  ~10 slots total spanning Mon-Fri
publications: 1
  state: 'DRAFT' (effective state ASSIGNING via past submission window)
  submission_start_at: now() - 14 days
  submission_end_at: now() - 7 days
  planned_active_from: now() + 7 days
availability_submissions: every employee submits ~60% of the slot-positions they're qualified for
assignments: 0 (admin runs auto-assign to fill)
```

#### Scenario: stress

Large-volume data to feel out UI density and rough performance. ~30-45 seconds.

```
users: bootstrap admin + 50 employees
positions: 8
user_positions: each employee qualified for 2-3 positions
template: 1, with slots:
  Mon-Fri × 4 daytime slots × 3 positions
  Mon-Sun × 1 evening slot × 2 positions
  ~80 slot-position rows total
publications: 4
  - 2 ENDED (historical, 4 weeks ago and 2 weeks ago)
  - 1 ACTIVE (current week, fully assigned)
  - 1 ASSIGNING (next week, 40% submissions, 0 assignments)
  Note: D2 invariant requires at most one publication with state != 'ENDED' at any time.
        Stress sets two ENDED + one ACTIVE + one ASSIGNING; the ACTIVE is the "live" non-ENDED
        publication, while the ASSIGNING one is in DRAFT-stored-state but ASSIGNING-effective-state
        because it's bypassing D2's effective-state rule via direct SQL. This is a deliberate
        seed shortcut and noted in the seed code as a comment.
availability_submissions: ~6 per employee in the ASSIGNING publication
assignments: full coverage of the ACTIVE publication (~40 rows)
shift_change_requests: 3 pending (1 swap, 1 give_direct, 1 give_pool)
```

**Note** on the D2 invariant in `stress`: spec promises "at most one publication with state != 'ENDED'." Direct SQL can technically violate this (the partial unique index on the `publications` table — let me check; if it's enforced at DB level, stress's setup needs a tweak). If the partial unique index is in place, stress will set ENDED states correctly so only one non-ENDED publication exists at any wall-clock instant. **Actionable in tasks.md**: confirm the index, adjust seed if needed.

### Makefile target

```makefile
SCENARIO ?= basic

seed:
	cd backend && go run ./cmd/seed --scenario=$(SCENARIO)
```

`make seed` → basic; `make seed SCENARIO=full` → full; `make seed SCENARIO=stress` → stress. Direct `cd backend && go run ./cmd/seed --scenario=full` works identically.

## Risks / Trade-offs

- **Risk**: developer runs `make seed` thinking it's additive, loses local data. Mitigation: prominent CLI output `WIPING database <name>` and a 1-second confirmation pause when stdout is a TTY (skipped in scripts). The pause is short enough to be a heuristic safety net without being annoying.
- **Risk**: seed and the application diverge as schema evolves. Mitigation: seed is exercised every dev session that uses smoke testing; drift surfaces fast.
- **Risk**: bulk INSERT into `assignments` triggers the `assignments_position_belongs_to_slot` BEFORE-INSERT trigger for each row. Performance: trivial for `basic` and `full`; potentially noticeable in `stress` (~40 trigger fires). Acceptable.
- **Trade-off**: seed wipes audit_logs too. If a developer wanted to preserve audit history across reseeds, they cannot. Acceptable — audit history is for production observability, not for dev-loop fixtures.

## Migration Plan

No data migration. Deployment:

1. Deploy backend code as usual; the new `cmd/seed` binary doesn't touch any existing path.
2. Documentation updated in README / docs to point at `make seed`.
3. Rollback: revert the change; the seed binary disappears; nothing else is affected.

## Open Questions

None. (The D2 partial-unique-index question above is captured as a task to confirm in implementation, not an open design question.)
