# Scheduling Subsystem — Behavior Spec

**Scope:** Templates, publications, availability, assignments, shift changes,
weekly roster. The "produce a weekly rota" half of Rota.

Related specs: `@openspec/specs/auth.md` covers sessions, admin/employee
roles, invitation and password-reset flows. `@openspec/specs/audit.md`
covers the append-only audit log that every mutating action in this
subsystem writes to.

---

## Background and problem

The rota project is a shift scheduling system for a ~100-person department
with multiple positions. Admins need to produce a weekly roster; employees
need a way to signal when they can work; both sides need the same source of
truth.

An earlier free-form "employee submits arbitrary intervals" model turned
out to be backwards: employees had no idea which slots were useful and
admins received unstructured data. The current model, confirmed with the
stakeholder, is:

1. Admin defines a **weekly template** (positions and shifts that repeat
   every week).
2. Admin **publishes** the template for a period — this opens a submission
   window.
3. Employees **tick the shifts they are available for** during the
   submission window.
4. After submissions close, admin **assigns** concrete employees to shifts
   (manually or via auto-assign).
5. Admin **publishes** the result, at which point employees can see the
   roster and request shift changes (swaps / gives).
6. Admin **activates** the publication on the intended start date; from
   then on the schedule is authoritative and shift-change requests stop.
7. Admin **ends** the publication when the period is over.

## Goals

- Employees only ever tick slots that are real, relevant, and that they are
  qualified for.
- Admins have a structured process with clear state transitions rather than
  ad-hoc manual work.
- Employees can self-serve simple shift adjustments (swap, give to someone,
  give to the pool) without bothering the admin.
- Exactly one publication is "current" at any time, so employee-facing
  reads never have to disambiguate.

## Non-goals

- Multi-timezone support. The department operates in one timezone.
- Leave / time-off / sick-day exceptions as first-class entities.
- Historical reporting, payroll, hours tracking.
- Notifications beyond the email notifications shift-change requests send.
- Multi-department or multi-organization support.

---

## Domain model

### Qualifications (`user_positions`)

Many-to-many link between users and positions, managed by admins.

| Column        | Type                       | Notes                                    |
|---------------|----------------------------|------------------------------------------|
| `user_id`     | `BIGINT`                   | FK `users.id`, cascade delete            |
| `position_id` | `BIGINT`                   | FK `positions.id`, cascade delete        |
| `created_at`  | `TIMESTAMPTZ`              | Default `NOW()`                          |

Primary key `(user_id, position_id)`. Qualification edits are done by
`PUT /users/{id}/positions`, which replaces the full set atomically.

An employee may only submit availability / accept shift changes for
positions they are qualified for. Admins bypass this check.

### Templates (`templates`, `template_shifts`)

A **template** is a named weekly blueprint. A **template shift** is one
cell inside the weekly grid — a position staffed on a specific weekday and
time window.

`templates` columns: `id`, `name`, `description`, `is_locked`,
`created_at`, `updated_at`.

`template_shifts` columns: `id`, `template_id`, `weekday`, `start_time`,
`end_time`, `position_id`, `required_headcount`, `created_at`,
`updated_at`. DB-level checks enforce `weekday BETWEEN 1 AND 7`
(Monday=1…Sunday=7), `end_time > start_time`, and
`required_headcount > 0`.

Index on `(template_id, weekday, start_time)` backs the canonical sort
order (week grid). Deleting a position used by any template shift is
blocked by `ON DELETE RESTRICT`.

Name is trimmed and limited to 100 code points; description to 500.
Shift times are stored as `TIME` and serialized as `HH:MM`; shifts do not
cross midnight (split such a shift into two rows).

### Publications (`publications`, `availability_submissions`)

A **publication** is one published instance of a template with its own
submission window, assignment set, and lifecycle.

Publication columns: `id`, `template_id`, `name`, `state`,
`submission_start_at`, `submission_end_at`, `planned_active_from`,
`activated_at` (nullable), `ended_at` (nullable), `created_at`,
`updated_at`.

DB-level invariants:

- `state ∈ { DRAFT, COLLECTING, ASSIGNING, PUBLISHED, ACTIVE, ENDED }`
  (CHECK constraint).
- `submission_start_at < submission_end_at <= planned_active_from`
  (CHECK constraint).
- Partial unique index `WHERE state != 'ENDED'` enforces the
  single-non-ENDED invariant (D2) at the database layer.
- `template_id` FK uses `ON DELETE RESTRICT`; a template that has ever been
  referenced cannot be deleted until all referencing publications are gone.

Creating a publication atomically sets the referenced template's
`is_locked = true` on first reference. Subsequent publications may
reference the same locked template.

An **availability submission** is one employee's tick that says "I am
available for this shift slot during this publication."

Columns: `id`, `publication_id`, `user_id`, `template_shift_id`,
`created_at`. Unique `(publication_id, user_id, template_shift_id)`.
Cascade delete from publication, user, or template_shift.

### Assignments (`assignments`)

Admin's final decision: "this user works this shift every week while the
publication is ACTIVE."

Columns: `id`, `publication_id`, `user_id`, `template_shift_id`,
`created_at`. Unique `(publication_id, user_id, template_shift_id)`.
Cascade delete from publication, user, or template_shift.

The number of assignments for a given `(publication_id,
template_shift_id)` *should* equal the shift's `required_headcount` but is
not hard-enforced — understaffed shifts are allowed (E4).

### Shift-change requests (`shift_change_requests`)

Created after a publication reaches `PUBLISHED`. Captures one proposed
reassignment that must be accepted by a counterpart or claimed from the
pool.

| Column                       | Type          | Notes                                              |
|------------------------------|---------------|----------------------------------------------------|
| `id`                         | `BIGSERIAL`   |                                                    |
| `publication_id`             | `BIGINT`      | FK, cascade delete                                 |
| `type`                       | `TEXT`        | CHECK `IN ('swap', 'give_direct', 'give_pool')`    |
| `requester_user_id`          | `BIGINT`      | FK `users.id`                                      |
| `requester_assignment_id`    | `BIGINT`      | Assignment being offered; no FK (see below)        |
| `counterpart_user_id`        | `BIGINT NULL` | Required for `swap` + `give_direct`; null for pool |
| `counterpart_assignment_id`  | `BIGINT NULL` | Required for `swap` only                           |
| `state`                      | `TEXT`        | See state machine below                            |
| `decided_by_user_id`         | `BIGINT NULL` | Who cancelled / approved / rejected                |
| `created_at`, `decided_at`   | `TIMESTAMPTZ` | `decided_at` null until terminal                   |
| `expires_at`                 | `TIMESTAMPTZ` | Set to `publication.planned_active_from`           |

State CHECK constraint: `IN ('pending', 'approved', 'rejected',
'cancelled', 'expired', 'invalidated')`.

Indexes cover `(publication_id, state, created_at DESC)`,
`(requester_user_id, state, created_at DESC)`, and
`(counterpart_user_id, state, created_at DESC)`.

Assignment columns are intentionally *not* FK-enforced so an admin edit
that deletes the referenced assignment doesn't cascade-delete pending
shift-change rows. Instead, staleness is detected lazily on approval
(see "Invariants").

---

## State machine

```
                 create (admin)
                      │
                      ▼
                    DRAFT
                      │  NOW() >= submission_start_at  (time-driven, lazy)
                      ▼
                 COLLECTING ──── employees tick / untick ────┐
                      │                                      │
                      │  NOW() >= submission_end_at           │
                      ▼                                      │
                  ASSIGNING ──── admin assigns / auto-assign ─┤
                      │                                      │
                      │  admin "publish"                     │
                      ▼                                      │
                  PUBLISHED ──── employees submit / resolve  │
                      │          shift-change requests       │
                      │  admin "activate"                    │
                      │  (bulk-expires pending requests)     │
                      ▼                                      │
                    ACTIVE ─── weekly roster is authoritative│
                      │                                      │
                      │  admin "end"                         │
                      ▼                                      │
                    ENDED                                    │
```

### Transitions

| From       | To          | Trigger                                                             |
|------------|-------------|---------------------------------------------------------------------|
| —          | DRAFT       | admin creates publication                                           |
| DRAFT      | COLLECTING  | time-driven; observed on read when `NOW() >= submission_start_at`   |
| COLLECTING | ASSIGNING   | time-driven; observed on read when `NOW() >= submission_end_at`     |
| ASSIGNING  | PUBLISHED   | admin `POST /publications/{id}/publish`                             |
| PUBLISHED  | ACTIVE      | admin `POST /publications/{id}/activate`                            |
| ACTIVE     | ENDED       | admin `POST /publications/{id}/end`                                 |
| DRAFT      | (deleted)   | admin `DELETE /publications/{id}`; non-DRAFT deletes are refused    |

Time-driven transitions are not advanced by a background job. The server
computes an **effective state** on every read (see "State resolution on
read") and lazily writes it through when a state-gated write arrives.

The manual transitions (publish, activate, end) are single-row conditional
`UPDATE`s with `sql.ErrNoRows` folded into a domain "not in expected
state" error, so concurrent clicks never double-transition.

The `activate` transition also bulk-updates every `pending`
shift-change request for the publication to `expired` inside the same
transaction (see "Invariants").

---

## Invariants

- **D2 — single non-ENDED publication.** At most one publication row has
  `state != 'ENDED'`. Enforced by a partial unique index in addition to
  the service-layer check. Attempted creation while another non-ENDED
  publication exists yields `PUBLICATION_ALREADY_EXISTS` (409).

- **Template lock (D1 / E1).** A template with `is_locked = true` is
  immutable: no shift create/update/delete, no template update, no
  template delete. Locking happens atomically on first publication
  reference, regardless of that publication's state (including DRAFT).
  Variants are produced by cloning (E2), which yields a fresh unlocked
  template whose name is `<original> (copy)` (truncated to fit 100 code
  points).

- **Availability window.** Availability submissions may only be created or
  deleted when the publication's *effective* state is COLLECTING. The
  repository writes honor a caller-supplied `PublicationState` override
  so the stored `state` is advanced from `DRAFT` or `COLLECTING` in the
  same transaction (lazy write-through).

- **Assignment window.** Assignment create / delete / auto-assign require
  effective state ASSIGNING. The assignment-board query also accepts
  ACTIVE (admins still need to see who works what once the publication is
  live).

- **Qualification.** Creating an availability submission, creating an
  assignment (admin), approving a swap, accepting a give, or claiming a
  pool request all check that the relevant user is qualified for the
  target shift's position. For swaps, the check is *mutual*: each party
  must be qualified for the other party's position.

- **Shift-change time conflict.** Before applying a swap or a give, the
  service recomputes the receiver's full weekly assignment set and
  rejects with `SHIFT_CHANGE_TIME_CONFLICT` if any two assignments would
  share a weekday and overlap in time (`a.start < b.end && b.start <
  a.end`).

- **Optimistic lock on shift changes.** `ApplySwap` / `ApplyGive` run
  inside a single transaction that re-reads both the request row and the
  referenced assignment row(s). If either assignment's
  `(id, publication_id, user_id)` no longer matches what the request
  captured, the repo returns `ErrShiftChangeAssignmentMiss`; the service
  then flips the request to `invalidated`. This is how admin edits to
  assignments "cascade-invalidate" pending shift-change requests without
  a FK or trigger.

- **Request expiry on activate.** Activating a publication is atomic
  with a bulk `UPDATE shift_change_requests SET state='expired' WHERE
  publication_id = $1 AND state='pending'`. Once ACTIVE, the schedule is
  authoritative; employees must go through the admin.

- **No self-target.** `swap` and `give_direct` reject
  `counterpart_user_id = requester_user_id` up-front
  (`SHIFT_CHANGE_SELF`); `give_pool` rejects the requester from claiming
  their own pool request at approval time.

---

## State resolution on read

Effective state is computed on every publication read. Pseudocode
(matching `model.ResolvePublicationState`):

```
effective_state(pub, now) =
  if pub.state ∈ (PUBLISHED, ACTIVE, ENDED):  pub.state
  elif now >= pub.submission_end_at:          ASSIGNING
  elif now >= pub.submission_start_at:        COLLECTING
  else:                                        DRAFT
```

Stored state is only advanced through writes: a submission
create/delete carries an optional `PublicationState` override, so
`UpsertSubmission` / `DeleteSubmission` set `state = 'COLLECTING'`
before applying the change if the row was still on `DRAFT`. The manual
transitions (publish / activate / end) use conditional `UPDATE`s and so
they do not need a separate resolver.

---

## Access and authorization

| Action                                      | Admin | Employee |
|---------------------------------------------|:-----:|:--------:|
| Manage positions (CRUD)                     | ✓     |          |
| List / replace a user's qualifications      | ✓     |          |
| List own qualified shifts during COLLECTING |       | ✓        |
| Template CRUD, clone, shift CRUD            | ✓     |          |
| Publication create / delete                 | ✓     |          |
| Publication publish / activate / end        | ✓     |          |
| Assignment board (get, create, delete)      | ✓     |          |
| Auto-assign                                 | ✓     |          |
| Tick own availability during COLLECTING     |       | ✓ *      |
| View current publication                    | ✓     | ✓        |
| View weekly roster during PUBLISHED/ACTIVE  | ✓     | ✓        |
| Create shift-change request                 |       | ✓ †      |
| Cancel own shift-change request             |       | ✓        |
| Reject / approve shift-change request       |       | ✓ ‡      |
| List / view shift-change requests           | ✓     | ✓ §      |
| List publication members                    | ✓     | ✓        |

Legend:
- \* Employees only see/tick `template_shifts` whose position is in their
  `user_positions` set.
- † Requester must own the `requester_assignment_id`; publication must be
  PUBLISHED.
- ‡ Reject: only the counterpart, and only for `swap`/`give_direct`.
  Approve: counterpart for `swap`/`give_direct`, any qualified non-self
  user for `give_pool`.
- § Employees see rows where they are requester, counterpart, or (for
  `give_pool`) any pending pool request. Admins see everything.

Admins do not tick their own availability through the standard flow —
they are assumed to be coordinators. Nothing prevents dual-role admins
from being assigned to shifts.

---

## Key flows

### Flow A — Setting up for the first time

1. Admin creates positions and users.
2. Admin sets each user's qualifications (`PUT /users/{id}/positions`).
3. Admin creates a template and fills its weekly grid of shifts.

### Flow B — Publishing a schedule

1. Admin creates a publication. Template locks. State: DRAFT.
2. `submission_start_at` passes. Effective state: COLLECTING. First
   employee submission advances stored state through write-through.
3. Employees log in, see the current publication, tick shifts they are
   qualified for.
4. `submission_end_at` passes. Effective state: ASSIGNING.
5. Admin opens the assignment board, sees candidates per shift
   (= users who ticked), assigns employees manually or clicks
   "auto-assign" which replaces the full assignment set.
6. Admin clicks "publish" → PUBLISHED. Roster becomes visible to
   employees; shift-change requests open.

### Flow C — Activating and ending

1. Admin clicks "activate" on the PUBLISHED publication. State → ACTIVE;
   all pending shift-change requests for that publication are bulk-expired
   in the same transaction.
2. Employees see the week-in-week-out roster via
   `GET /roster/current`.
3. Admin clicks "end" → ENDED. `/publications/current` and
   `/roster/current` start returning null. A new publication can now be
   created.

### Flow D — Shift changes (PUBLISHED only)

1. Alice is assigned to "Mon 09–13 Cashier." She opens the roster,
   chooses one of:
   - **swap** with Bob's "Tue 14–18 Cashier" — Bob must be qualified for
     Cashier, Alice must be qualified for Bob's position, neither ends up
     with an overlapping weekly schedule.
   - **give_direct** to Carol — Carol must be qualified for Cashier and
     not already overlap.
   - **give_pool** — any qualified user can later claim it.
2. The server persists a `pending` row with
   `expires_at = publication.planned_active_from` and sends an email to
   the counterpart for `swap` / `give_direct` (no email for pool).
3. Counterpart (or any qualified user, for pool) approves → the repo
   atomically updates the `assignments` table and sets request
   `state = 'approved'`. If the captured assignment row no longer
   exists (admin edited assignments after the request was created), the
   repo returns a miss, the service marks the request `invalidated`, and
   the client sees `SHIFT_CHANGE_INVALIDATED`.
4. Read paths also mark a stale pending request `expired` if
   `NOW() > expires_at`, so the counterpart's list stays tidy even
   before the activate transaction runs.

---

## API surface

All mutating endpoints write to the audit log (see `@openspec/specs/audit.md`).
All endpoints below are served under the JSON API root; `RequireAdmin`
and `RequireAuth` are the session-based middlewares described in
`@openspec/specs/auth.md`.

### Qualifications

| Method | Path                          | Purpose                              | Middleware    |
|--------|-------------------------------|--------------------------------------|---------------|
| GET    | `/users/{id}/positions`       | List a user's positions              | RequireAdmin  |
| PUT    | `/users/{id}/positions`       | Replace a user's positions           | RequireAdmin  |

### Templates

| Method | Path                                          | Purpose                      | Middleware   | Lock gate |
|--------|-----------------------------------------------|------------------------------|--------------|-----------|
| GET    | `/templates`                                  | List (paginated)             | RequireAdmin |           |
| POST   | `/templates`                                  | Create                       | RequireAdmin |           |
| GET    | `/templates/{id}`                             | Get with shifts              | RequireAdmin |           |
| PUT    | `/templates/{id}`                             | Update name/description      | RequireAdmin | unlocked  |
| DELETE | `/templates/{id}`                             | Delete                       | RequireAdmin | unlocked  |
| POST   | `/templates/{id}/clone`                       | Clone (produces unlocked)    | RequireAdmin |           |
| POST   | `/templates/{id}/shifts`                      | Create shift                 | RequireAdmin | unlocked  |
| PATCH  | `/templates/{id}/shifts/{shift_id}`           | Update shift                 | RequireAdmin | unlocked  |
| DELETE | `/templates/{id}/shifts/{shift_id}`           | Delete shift                 | RequireAdmin | unlocked  |

### Publications (admin)

| Method | Path                                               | Purpose                                   | Effective-state gate      |
|--------|----------------------------------------------------|-------------------------------------------|---------------------------|
| GET    | `/publications`                                    | List (paginated)                          |                           |
| POST   | `/publications`                                    | Create (locks template)                   | no other non-ENDED exists |
| GET    | `/publications/{id}`                               | Detail                                    |                           |
| DELETE | `/publications/{id}`                               | Delete                                    | DRAFT only                |
| GET    | `/publications/{id}/assignment-board`              | Admin pool + assignments per shift        | ASSIGNING or ACTIVE       |
| POST   | `/publications/{id}/auto-assign`                   | Replace assignments via MCMF solver       | ASSIGNING                 |
| POST   | `/publications/{id}/assignments`                   | Create one assignment                     | ASSIGNING                 |
| DELETE | `/publications/{id}/assignments/{assignment_id}`   | Delete one assignment                     | ASSIGNING                 |
| POST   | `/publications/{id}/publish`                       | ASSIGNING → PUBLISHED                     | ASSIGNING                 |
| POST   | `/publications/{id}/activate`                      | PUBLISHED → ACTIVE; bulk-expires requests | PUBLISHED                 |
| POST   | `/publications/{id}/end`                           | ACTIVE → ENDED                            | ACTIVE                    |

All publication admin endpoints require `RequireAdmin`.

### Publications (employee-facing)

| Method | Path                                            | Purpose                                    | Gate                              |
|--------|-------------------------------------------------|--------------------------------------------|-----------------------------------|
| GET    | `/publications/current`                         | Currently non-ENDED publication            | RequireAuth                       |
| GET    | `/publications/{id}/roster`                     | Full weekly roster                         | RequireAuth; PUBLISHED or ACTIVE  |
| GET    | `/roster/current`                               | Roster of the current publication (or empty) | RequireAuth; PUBLISHED or ACTIVE |
| GET    | `/publications/{id}/shifts/me`                  | Shifts the viewer is qualified for         | RequireAuth; COLLECTING           |
| GET    | `/publications/{id}/submissions/me`             | Viewer's own ticked shift IDs              | RequireAuth                       |
| POST   | `/publications/{id}/submissions`                | Tick a shift                               | RequireAuth; COLLECTING           |
| DELETE | `/publications/{id}/submissions/{shift_id}`     | Un-tick                                    | RequireAuth; COLLECTING           |
| GET    | `/publications/{id}/members`                    | Users assigned in the publication          | RequireAuth                       |

### Shift changes

All `RequireAuth`.

| Method | Path                                                           | Purpose                                    | Gate        |
|--------|----------------------------------------------------------------|--------------------------------------------|-------------|
| POST   | `/publications/{id}/shift-changes`                             | Create swap / give_direct / give_pool      | PUBLISHED   |
| GET    | `/publications/{id}/shift-changes`                             | List filtered by audience                  |             |
| GET    | `/publications/{id}/shift-changes/{request_id}`                | Detail                                     |             |
| POST   | `/publications/{id}/shift-changes/{request_id}/approve`        | Counterpart approve or pool claim          | PUBLISHED   |
| POST   | `/publications/{id}/shift-changes/{request_id}/reject`         | Counterpart reject (swap / give_direct)    |             |
| POST   | `/publications/{id}/shift-changes/{request_id}/cancel`         | Requester cancel                           |             |
| GET    | `/users/me/notifications/unread-count`                         | Pending count for viewer as counterpart    |             |

---

## Error codes

HTTP status is the handler-layer mapping; error code is the JSON
`error.code` field.

| Code                         | HTTP | Emitted when                                                                 |
|------------------------------|------|------------------------------------------------------------------------------|
| `INVALID_REQUEST`            | 400  | Malformed body / path / query or generic `ErrInvalidInput`                   |
| `INVALID_PUBLICATION_WINDOW` | 400  | Window does not satisfy `start < end <= planned_active_from`                 |
| `SHIFT_CHANGE_INVALID_TYPE`  | 400  | Unknown request type or wrong counterpart fields for the type                |
| `SHIFT_CHANGE_SELF`          | 400  | Counterpart / claimer is the requester themselves                            |
| `PUBLICATION_NOT_FOUND`      | 404  | No row, or effective state resolution requested for a missing publication   |
| `TEMPLATE_NOT_FOUND`         | 404  | Referenced template missing                                                  |
| `TEMPLATE_SHIFT_NOT_FOUND`   | 404  | Shift not found for the given template                                      |
| `USER_NOT_FOUND`             | 404  | Referenced user missing                                                      |
| `SHIFT_CHANGE_NOT_FOUND`     | 404  | Request missing or hidden from the viewer                                    |
| `NOT_QUALIFIED`              | 403  | Employee attempts a submission/approval for a position they lack             |
| `SHIFT_CHANGE_NOT_OWNER`     | 403  | Caller is not the request's requester / counterpart / claimer                |
| `SHIFT_CHANGE_NOT_QUALIFIED` | 403  | Swap / give counterpart is not mutually qualified                            |
| `PUBLICATION_ALREADY_EXISTS` | 409  | Create request violates the single-non-ENDED invariant                       |
| `PUBLICATION_NOT_DELETABLE`  | 409  | Delete request on a non-DRAFT publication                                    |
| `PUBLICATION_NOT_COLLECTING` | 409  | Submission write outside COLLECTING                                          |
| `PUBLICATION_NOT_ASSIGNING`  | 409  | Assignment write / publish outside ASSIGNING                                 |
| `PUBLICATION_NOT_PUBLISHED`  | 409  | Activate outside PUBLISHED; shift-change write outside PUBLISHED             |
| `PUBLICATION_NOT_ACTIVE`     | 409  | End outside ACTIVE; roster fetched for a publication that is not viewable   |
| `USER_DISABLED`              | 409  | Admin tries to assign a disabled user                                        |
| `SHIFT_CHANGE_TIME_CONFLICT` | 409  | Applying the change would create an overlapping weekly assignment            |
| `SHIFT_CHANGE_NOT_PENDING`   | 409  | Approve / reject / cancel on a terminal request                              |
| `SHIFT_CHANGE_EXPIRED`       | 409  | Approve / reject / cancel on a request past `expires_at`                     |
| `SHIFT_CHANGE_INVALIDATED`   | 409  | Approve surfaces that the captured assignment row is gone or reassigned      |
| `INTERNAL_ERROR`             | 500  | Anything else                                                                |

---

## Auto-scheduling algorithm

`POST /publications/{id}/auto-assign` runs a min-cost max-flow solver
over the candidate pool and then replaces the entire assignment set for
the publication inside one transaction (so a partial result is never
observed).

Graph shape:

- Source `s`.
- For each user with at least one candidacy, build per-weekday overlap
  groups — maximal sets of shifts the user ticked that pairwise overlap
  on the same weekday. Each group gets its own "group node"; a user can
  take at most one shift per group. The user also has up to
  `min(#groups, total_demand)` "slot" nodes sitting between `s` and a
  central "employee" node.
- One node per shift; shifts connect to sink `t` with capacity
  `required_headcount` and a negative coverage bonus.
- Capacities are 1 on all user-side edges; costs on slot edges grow
  linearly with the slot index so distributing shifts across employees is
  preferred over piling many on one person.

The solver maximises total flow at minimum cost. Because the coverage
bonus is large and negative (`-2 * total_demand`), the algorithm greedily
fills demand first; the slot-index cost term then spreads work. Capacity
`1` from group to shift prevents same-user double-booking within an
overlap group.

The solver is intentionally simple. It does *not* optimise for fairness
over time, seniority, or any notion of preference weighting — that is
out of scope. Admins can still hand-edit any assignment afterward.

---

## Decisions log

- **D1.** Templates are immutable once referenced by any publication
  (including DRAFT). Variants are produced by cloning.
- **D2.** At most one publication may have `state != ENDED` at any time.
  Enforced both in the service layer and by a partial unique index.
- **D3.** Single department timezone. No per-user timezones.
- **D4.** Weekly concrete shifts during PUBLISHED/ACTIVE are computed on
  read from `(publication, assignments)` — they are not materialized
  per week.
- **D5.** PUBLISHED is a distinct state between ASSIGNING and ACTIVE.
  It opens the roster to employees and opens the shift-change market
  without the schedule yet being "live." Activation is a separate click
  and is the point of no return for shift-change requests.
- **D6.** Shift-change requests hold `requester_assignment_id` /
  `counterpart_assignment_id` as plain BIGINTs, not FKs. Stale references
  are detected and resolved to `invalidated` at approval time by the
  atomic Apply* repo method.
- **D7.** `expires_at` defaults to `publication.planned_active_from`,
  but activation is the authoritative trigger: activating a publication
  bulk-expires every pending request in the same transaction.
- **E1.** Template locking triggers on first publication reference, even
  in DRAFT.
- **E2.** Template cloning is an explicit admin action producing a new,
  unlocked template whose name is suffixed with `(copy)` (truncated to
  fit 100 code points).
- **E3.** DRAFT → COLLECTING → ASSIGNING transitions are time-driven and
  resolved on read; submission writes lazily advance the stored state.
- **E4.** PUBLISHED/ACTIVE allow understaffed shifts (fewer assignments
  than `required_headcount`). The admin UI surfaces it as a warning.
- **E5.** No system-enforced cap on assignments per employee per week;
  the admin is responsible.
- **E6.** Publications can be deleted only while effective state is
  DRAFT.
- **E7.** The employee weekly roster shows the **full** roster (every
  shift, every assigned user) with the viewer's own shifts highlighted.
- **E8.** Swap / give pre-application rejects schedule overlaps but not
  understaffing — it is acceptable for the receiver to take over a shift
  that leaves the original position short-handed in aggregate, because
  per-shift `required_headcount` is already advisory (E4).
- **E9.** `give_pool` requests have no email notification at creation —
  they are not targeted at a specific recipient — but the requester
  still gets a resolution email once someone claims the shift.
- **E10.** `CountPendingForViewer` ignores `give_pool` because those
  requests have no specific recipient; a pending pool offer should not
  show up in a personal "you have N requests waiting" badge.
