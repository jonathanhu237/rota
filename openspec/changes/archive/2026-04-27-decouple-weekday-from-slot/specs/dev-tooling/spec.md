## MODIFIED Requirements

### Requirement: Local-development data seeding command

The project SHALL provide a `make seed` command (and equivalent `go run ./backend/cmd/seed`) that resets the configured Postgres database to one of four named scenarios in seconds. The command SHALL refuse to run when the configured `AppEnv` resolves to `production` and SHALL print a clear "WIPING database" banner before truncating tables.

The seeded data SHALL satisfy the existing schema constraints (foreign keys, UNIQUE indexes, the `template_slot_weekdays` overlap trigger, the `assignments_position_belongs_to_slot` trigger). The seed binary SHALL compute bcrypt hashes for seeded user passwords (no plaintext or fake hashes in the database), and SHALL NOT emit application-level audit events while inserting.

#### Scenario: make seed wipes and reseeds with the basic scenario

- **GIVEN** a developer's local Postgres with arbitrary prior state
- **WHEN** the developer runs `make seed`
- **THEN** the command prints `WIPING database <db>@<host>:<port>`
- **AND** the data tables (`users`, `positions`, `templates`, `template_slots`, `template_slot_weekdays`, `template_slot_positions`, `user_positions`, `publications`, `availability_submissions`, `assignments`, `shift_change_requests`, `user_setup_tokens`, `audit_logs`) are TRUNCATEd with `RESTART IDENTITY CASCADE`
- **AND** the bootstrap admin and 5 placeholder employees are inserted with bcrypt-hashed `pa55word`, all with `status='active'`
- **AND** 3 positions and 1 empty template are inserted
- **AND** no publications, submissions, or assignments are created
- **AND** the command exits 0

#### Scenario: make seed SCENARIO=full provides ASSIGNING-state data

- **WHEN** the developer runs `make seed SCENARIO=full`
- **THEN** the basic scenario data is present
- **AND** the template has approximately 6 logical slots whose combined weekday coverage spans Mon-Fri, each with multi-position composition
- **AND** each employee is qualified for 2 positions
- **AND** exactly one publication exists in effective state `ASSIGNING` (i.e., `submission_end_at` in the past, `planned_active_from` in the future)
- **AND** roughly 60% of qualified `(slot, weekday)` cells have an `availability_submissions` row per employee
- **AND** zero assignments exist (so the developer can immediately invoke auto-assign)

#### Scenario: make seed SCENARIO=stress provides large-volume multi-publication data

- **WHEN** the developer runs `make seed SCENARIO=stress`
- **THEN** approximately 50 employees, 8 positions, and a template whose slots collectively claim ~80 `(slot, weekday)` cells with multi-position composition are present
- **AND** four publications exist: three `ENDED` fixture/historical publications and one `ACTIVE` publication with assignment coverage
- **AND** the resulting state respects the single-non-ENDED publication invariant (D2)
- **AND** at least one pending shift-change request exists per type (swap, give_direct, give_pool)

#### Scenario: make seed SCENARIO=realistic provides anonymized real-cohort data

- **WHEN** the developer runs `make seed SCENARIO=realistic`
- **THEN** the bootstrap admin plus 42 employees are inserted with anonymized identifiers (`employee01..42` slugs, `员工 1..42` Chinese display names, `employee01@example.com..employee42@example.com` emails) and bcrypt-hashed `pa55word`
- **AND** 4 positions are inserted: `前台负责人`, `前台助理`, `外勤负责人`, `外勤助理`
- **AND** each employee is assigned a deterministic archetype (`前台-senior`, `前台-junior`, `外勤-senior`, `外勤-junior`) drawn from a fixed RNG seed compiled into the binary, with approximately 75% in the `前台` domain and 25% in the `外勤` domain, and at least one senior of each domain available on every `(weekday, domain)` cell where the source data has any availability (cells with zero source-data availability — e.g., Sunday daytime in the front domain — remain uncovered, which is intentional and exercises the empty-slot path of auto-assign)
- **AND** `user_positions` rows are inserted so seniors are qualified for both their domain's lead and assistant positions, juniors only for the assistant position
- **AND** 1 template named "Realistic Rota" is inserted with 5 `template_slots` (one per time block: 09:00-10:00, 10:00-12:00, 13:30-16:10, 16:10-18:00, 19:00-21:00)
- **AND** 35 `template_slot_weekdays` rows are inserted (5 slots × 7 weekdays each, since every time block runs all seven days in this cohort)
- **AND** 10 `template_slot_positions` rows are inserted: each daytime slot requires `{前台负责人 × 1, 前台助理 × 2}` and the evening slot requires `{外勤负责人 × 1, 外勤助理 × 1}` (composition stored once per slot, not duplicated per weekday)
- **AND** exactly one publication exists in effective state `ASSIGNING`
- **AND** `availability_submissions` rows are inserted from the embedded weekday vectors, each row carrying `(slot_id, weekday)`, dropping any submission whose slot domain does not match the employee's archetype domain
- **AND** zero assignments exist (so the developer can immediately invoke auto-assign)
- **AND** re-running the scenario yields the same logical scenario shape (same anonymized roster, archetypes, slots, weekday membership, position requirements, and availability vectors), excluding runtime-generated timestamps and bcrypt salts

#### Scenario: seed refuses to run in production

- **GIVEN** an environment where `cfg.AppEnv` resolves to `"production"`
- **WHEN** any invocation of `make seed` or `go run ./backend/cmd/seed` is attempted
- **THEN** the binary exits non-zero before opening the database
- **AND** stderr contains a message naming "production" and "refusing"
- **AND** no `TRUNCATE` is executed
- **AND** no row is modified
