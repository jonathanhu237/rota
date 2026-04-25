## ADDED Requirements

### Requirement: Local-development data seeding command

The project SHALL provide a `make seed` command (and equivalent `go run ./backend/cmd/seed`) that resets the configured Postgres database to one of three named scenarios in seconds. The command SHALL refuse to run when the configured `AppEnv` resolves to `production` and SHALL print a clear "WIPING database" banner before truncating tables.

The seeded data SHALL satisfy the existing schema constraints (foreign keys, UNIQUE indexes, GIST exclusion on `template_slots`, the `assignments_position_belongs_to_slot` trigger). The seed binary SHALL compute bcrypt hashes for seeded user passwords (no plaintext or fake hashes in the database), and SHALL NOT emit application-level audit events while inserting.

#### Scenario: make seed wipes and reseeds with the basic scenario

- **GIVEN** a developer's local Postgres with arbitrary prior state
- **WHEN** the developer runs `make seed`
- **THEN** the command prints `WIPING database <db>@<host>:<port>`
- **AND** the data tables (`users`, `positions`, `templates`, `template_slots`, `template_slot_positions`, `user_positions`, `publications`, `availability_submissions`, `assignments`, `shift_change_requests`, `user_setup_tokens`, `audit_logs`) are TRUNCATEd with `RESTART IDENTITY CASCADE`
- **AND** the bootstrap admin and 5 placeholder employees are inserted with bcrypt-hashed `pa55word`, all with `status='active'`
- **AND** 3 positions and 1 empty template are inserted
- **AND** no publications, submissions, or assignments are created
- **AND** the command exits 0

#### Scenario: make seed SCENARIO=full provides ASSIGNING-state data

- **WHEN** the developer runs `make seed SCENARIO=full`
- **THEN** the basic scenario data is present
- **AND** the template has approximately 10 slots spanning Mon-Fri with multi-position composition
- **AND** each employee is qualified for 2 positions
- **AND** exactly one publication exists in effective state `ASSIGNING` (i.e., `submission_end_at` in the past, `planned_active_from` in the future)
- **AND** roughly 60% of qualified `(slot, position)` pairs have an `availability_submissions` row per employee
- **AND** zero assignments exist (so the developer can immediately invoke auto-assign)

#### Scenario: make seed SCENARIO=stress provides large-volume multi-publication data

- **WHEN** the developer runs `make seed SCENARIO=stress`
- **THEN** approximately 50 employees, 8 positions, and a template with ~80 slot-positions are present
- **AND** four publications exist: three `ENDED` fixture/historical publications and one `ACTIVE` publication with assignment coverage
- **AND** the resulting state respects the single-non-ENDED publication invariant (D2)
- **AND** at least one pending shift-change request exists per type (swap, give_direct, give_pool)

#### Scenario: seed refuses to run in production

- **GIVEN** an environment where `cfg.AppEnv` resolves to `"production"`
- **WHEN** any invocation of `make seed` or `go run ./backend/cmd/seed` is attempted
- **THEN** the binary exits non-zero before opening the database
- **AND** stderr contains a message naming "production" and "refusing"
- **AND** no `TRUNCATE` is executed
- **AND** no row is modified
