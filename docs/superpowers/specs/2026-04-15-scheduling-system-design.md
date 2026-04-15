# Scheduling System — Design Spec

**Date:** 2026-04-15
**Status:** Approved for implementation
**Scope:** Whole scheduling subsystem (templates, publications, availability, assignment, active weekly view)

---

## Background and problem

The rota project is a shift scheduling system for a ~100-person department with
multiple positions. Admins need to produce a weekly roster; employees need a
way to signal when they can work; both sides need the same source of truth.

An earlier iteration built a free-form "employee submits arbitrary free time
intervals" feature. That turned out to be backwards: employees had no idea
which slots were useful, and admins received unstructured data that was
expensive to turn into a roster. This spec replaces that approach.

The correct model, confirmed with the stakeholder, is:

1. Admin defines a **weekly template** (positions and shifts that repeat every
   week).
2. Admin **publishes** the template for a period — this opens a submission
   window.
3. Employees **tick the shifts they are available for** during the submission
   window.
4. After submissions close, admin **assigns** concrete employees to concrete
   shift slots from the pool of candidates.
5. Admin **activates** the publication; from that point on, assigned employees
   work those shifts every week until the admin ends the publication or
   publishes a new one.

## Goals

- Employees only ever tick slots that are real, relevant, and they are
  qualified for.
- Admins have a structured process with clear state transitions rather than
  ad-hoc manual work.
- The data model is simple enough that a future auto-scheduling algorithm
  can plug in without reshuffling tables.
- Only one active publication at a time so employees never see a conflicting
  or ambiguous "what am I supposed to do this week."

## Non-goals (for this whole subsystem)

- Multi-timezone support. The department operates in one timezone.
- Leave / time-off / sick-day exceptions. Handled out-of-band until a future
  iteration.
- Swap requests, transfers, shift marketplaces between employees.
- Notifications (email / push) on state transitions.
- Historical reporting, payroll, hours tracking.
- Multi-department or multi-organization support.

---

## Domain model

### Entities

**User** (existing): the employee or admin. Already has `is_admin`.

**Position** (existing): a role that needs to be staffed (e.g. "Front Desk",
"Cashier").

**UserPosition** (new): many-to-many between User and Position. Says "this
user is qualified to work this position." Admin-managed.

- Columns: `user_id`, `position_id`, `created_at`
- Primary key: `(user_id, position_id)`
- FKs cascade on delete of user or position.

**Template** (new): a named weekly blueprint. Contains a set of shifts.

- Columns: `id`, `name`, `description`, `is_locked`, `created_at`, `updated_at`
- `is_locked` flips to `true` the first time any Publication references it.
  A locked template is immutable (no edits, no shift changes, no deletion).
  Admin must clone it to produce a variant.

**TemplateShift** (new): one shift slot inside a template.

- Columns: `id`, `template_id`, `weekday`, `start_time`, `end_time`,
  `position_id`, `required_headcount`, `created_at`, `updated_at`
- `weekday`: integer 1–7, Monday = 1 through Sunday = 7.
- `start_time`, `end_time`: `TIME` (no date component), interpreted in the
  department timezone. `end_time > start_time` (shifts do not cross midnight
  — split such cases into two shifts).
- `required_headcount`: positive integer, the exact number of people the
  shift needs. Admin may end up assigning fewer at activation time (see E4);
  this field is the target, not a hard constraint.
- `position_id` FK to positions. Deleting a position used by any TemplateShift
  is blocked (either restrict at the DB layer or surface a conflict error).

**Publication** (new): one published instance of a template, with a
submission window and an activation lifecycle.

- Columns: `id`, `template_id`, `name`, `submission_start_at`,
  `submission_end_at`, `planned_active_from`, `state`, `created_at`,
  `activated_at` (nullable), `ended_at` (nullable)
- `state`: one of `DRAFT`, `COLLECTING`, `ASSIGNING`, `ACTIVE`, `ENDED`.
- `submission_start_at`, `submission_end_at`, `planned_active_from` are
  timestamps with time zone. They are required on creation — a publication
  cannot be half-configured.
- `activated_at` is set when admin clicks "activate." `ended_at` is set when
  admin ends the publication (or when a new publication is created and
  activated, implicitly ending the previous one — see "activation rules").
- `planned_active_from` is informational only. It is displayed in the UI as
  "expected start" but does not trigger the transition.

**AvailabilitySubmission** (new): an employee's tick that says "I am
available for this shift slot during this publication."

- Columns: `id`, `publication_id`, `user_id`, `template_shift_id`, `created_at`
- Unique constraint: `(publication_id, user_id, template_shift_id)`.
- FK cascade on delete of publication or user. (If the template and thus the
  shift is immutable once published, deleting a template shift should be
  impossible anyway — see Template.is_locked.)

**Assignment** (new): admin's final decision — "this user will work this
shift slot every week while this publication is ACTIVE."

- Columns: `id`, `publication_id`, `user_id`, `template_shift_id`, `created_at`
- Unique constraint: `(publication_id, user_id, template_shift_id)`.
- The number of assignments for a given `(publication_id, template_shift_id)`
  SHOULD equal the shift's `required_headcount`, but is not hard-enforced.

### Diagram (textual)

```
User ─── UserPosition ───┐
  │                      │
  │                      ▼
  │                   Position ◄──── TemplateShift ──── Template
  │                                    ▲
  ├── AvailabilitySubmission ──────────┤
  │                                    │
  └── Assignment ──────────────────────┘
         │
         ▼
     Publication ── template_id ──► Template
```

AvailabilitySubmission and Assignment both reference a
`(publication_id, template_shift_id)` pair. Because templates are locked
after first use, this reference is stable for the lifetime of the publication.

---

## State machine

```
           created with all required fields
                    │
                    ▼
                 DRAFT
                    │  submission_start_at reached (auto)
                    ▼
              COLLECTING ───────────┐
                    │               │  employees tick/untick
                    │  submission_end_at reached (auto)
                    ▼
              ASSIGNING ────────────┐
                    │               │  admin assigns employees
                    │  admin clicks "activate" (manual)
                    ▼
                 ACTIVE ────────────┐
                    │               │  weekly view served from assignments
                    │  admin clicks "end" OR new publication activates
                    ▼
                 ENDED
```

### Transition rules

- **→ DRAFT**: the only way to enter DRAFT is creating a new publication.
  Creation requires `template_id`, `name`, `submission_start_at`,
  `submission_end_at`, `planned_active_from`. All three timestamps must be
  in chronological order (`submission_start_at < submission_end_at <=
  planned_active_from`). Creation atomically locks the referenced template:
  if `is_locked = false`, flip it to `true`; if it is already locked (because
  another publication already referenced it), reuse it as-is. A locked
  template is perfectly valid to reference — you just can't edit it anymore.

- **DRAFT → COLLECTING**: automatic when `NOW() >= submission_start_at`. The
  transition is observed on any read — we do not need a background job for
  this phase (see "state resolution" below).

- **COLLECTING → ASSIGNING**: automatic when `NOW() >= submission_end_at`.
  Same observation-on-read model.

- **ASSIGNING → ACTIVE**: manual. Admin clicks "activate" in the UI. Server
  sets `state = ACTIVE`, `activated_at = NOW()`. Precondition: no other
  publication is currently in `ACTIVE`. If there is one, the admin is shown a
  confirmation: "activating this will end publication X" — on confirm, the
  server atomically transitions both (X → ENDED with `ended_at = NOW()`,
  this one → ACTIVE).

- **ACTIVE → ENDED**: manual. Admin clicks "end publication" — server sets
  `state = ENDED`, `ended_at = NOW()`.

### Single-non-ENDED invariant (D2)

Only one publication may exist with `state ≠ ENDED` at any time. Enforced
at creation: trying to create a new publication while an unfinished one
exists is rejected, unless the admin explicitly chooses to end the existing
one as part of the creation flow.

This keeps employee-facing reads simple: "the current publication" is
unambiguously defined as `SELECT * FROM publications WHERE state != 'ENDED'
LIMIT 1`.

### State resolution on read

DRAFT → COLLECTING and COLLECTING → ASSIGNING are time-driven. Rather than
running a scheduler, the server resolves the *effective* state on every
read:

```
effective_state(pub) =
  if pub.state in (ACTIVE, ENDED):       pub.state
  elif NOW() >= pub.submission_end_at:   ASSIGNING
  elif NOW() >= pub.submission_start_at: COLLECTING
  else:                                  DRAFT
```

The stored `state` is lazily advanced on write paths that care (submission
create/delete is rejected if effective state ≠ COLLECTING; assignment writes
are rejected if effective state ≠ ASSIGNING). This avoids a cron job and
avoids the "what if the clock drifted" failure mode.

---

## Access and authorization

| Action                                    | Admin | Employee |
|-------------------------------------------|:-----:|:--------:|
| Manage positions                          | ✓     |          |
| Manage user↔position qualification        | ✓     |          |
| Manage templates (create/edit/delete)     | ✓     |          |
| Clone template                            | ✓     |          |
| Create / end publication                  | ✓     |          |
| Activate publication                      | ✓     |          |
| Assign employees to shifts                | ✓     |          |
| View current publication                  | ✓     | ✓        |
| Tick own availability during COLLECTING   |       | ✓ (1)    |
| View this week's full roster during ACTIVE| ✓     | ✓        |

(1) Employees can only tick `TemplateShift` rows whose `position_id` is in
their `UserPosition` set.

Admins do not tick their own availability through this flow — they are
assumed to be coordinators, not workers in the schedule. If an admin is
actually also a worker (dual role), they can do so, but this spec does not
treat that as a special case.

---

## Key interaction flows

### Flow A — Setting up for the first time

1. Admin creates positions (existing feature).
2. Admin creates users (existing feature).
3. Admin opens "Qualifications" page and ticks which positions each user can
   do. (PR 1)
4. Admin opens "Templates" page, creates a template, adds shifts. (PR 2)

### Flow B — Publishing a schedule

1. Admin creates a publication: picks a template, names the publication,
   sets `submission_start_at`, `submission_end_at`, `planned_active_from`.
   Referenced template is locked. (PR 3)
2. Time passes. Effective state advances DRAFT → COLLECTING on its own.
3. During COLLECTING, employees log in, see the current publication, see
   every `TemplateShift` they are qualified for grouped by weekday, tick the
   ones they are available for. Ticks can be toggled freely while
   COLLECTING. (PR 3)
4. `submission_end_at` passes. Effective state → ASSIGNING.
5. Admin opens the assignment view: for each `TemplateShift`, they see the
   list of candidates (users who ticked) and can assign up to (or even
   beyond) `required_headcount`. (PR 4)
6. Admin clicks "activate." State → ACTIVE; `activated_at` set. (PR 4)

### Flow C — Week in the life during ACTIVE

1. Employee logs in, lands on dashboard, sees "This week's roster" card.
   (PR 4)
2. Clicking through shows the full weekly grid (all shifts across the week,
   assigned employees per shift, own shifts highlighted). Decision E7/B.
3. When the admin decides the period is over, they click "end publication."
   State → ENDED. There is now no current publication until the admin
   creates a new one.

---

## Decomposition into PRs

Each PR is one full PLAN → SUMMARY → REVIEW → commit cycle.

**PR 1 — Qualification management.**
Backend: `user_positions` table, CRUD for admin. Frontend: admin page to
view a user and toggle which positions they can do. Small, independent,
unblocks everything downstream.

**PR 2 — Template CRUD.**
Backend: `templates`, `template_shifts` tables, CRUD for admin, template
cloning. Frontend: template list, template editor with shift management.
Independent of PR 1 (templates don't yet care about employees).

**PR 3 — Publication lifecycle + employee submission.**
Backend: `publications`, `availability_submissions` tables, creation
endpoint with the single-non-ENDED invariant and template locking, effective
state resolution, submission endpoints. Frontend: admin "publish" form,
employee COLLECTING view with per-weekday grid of tickable shifts filtered
by qualifications. State-gated writes.

**PR 4 — Assignment + activation + weekly view + end.**
Backend: `assignments` table, admin assignment endpoints, activate/end
transitions with atomic "end previous on activate" behavior, weekly-view
query. Frontend: admin assignment UI (candidate pool per shift, assign),
activation button with confirmation, employee weekly roster view
(option B — full grid), admin end-publication button.

**PR 5 (optional, later) — Auto-scheduling algorithm.**
An "auto-suggest" button in PR 4's assignment UI that fills assignments
from the candidate pool using some fairness heuristic. Entirely additive.

PRs 1 and 2 can be done in either order. PR 3 depends on both. PR 4 depends
on PR 3. PR 5 depends on PR 4.

---

## Decisions log

Captured during brainstorming with the stakeholder; listed here so future
readers know which things were deliberate and which are negotiable.

- **D1.** Templates are immutable once referenced by any publication
  (including one in DRAFT). Variants are produced by cloning.
- **D2.** At most one publication may have `state ≠ ENDED` at any time.
- **D3.** Single department timezone. No per-user timezones.
- **D4.** Weekly concrete shifts during ACTIVE are computed on read from
  `(publication, assignments, current_date)` — they are not materialized
  per week.
- **E1.** Template locking triggers on first publication reference, even
  in DRAFT.
- **E2.** Template cloning is an explicit admin action producing a new,
  unlocked template.
- **E3.** DRAFT → COLLECTING → ASSIGNING transitions are time-driven and
  resolved on read (no background job).
- **E4.** ACTIVE allows understaffed shifts (fewer assignments than
  `required_headcount`). Admin sees a visual warning.
- **E5.** No system-enforced cap on assignments per employee per week.
  Admin is responsible.
- **E6.** Publications can be deleted only while `state = DRAFT`.
- **E7.** Employee weekly view during ACTIVE shows the **full** roster
  (all assigned employees on every shift), with own shifts highlighted.

## Open questions

None at design time. Implementation questions that will surface in each PR
— error code names, exact pagination, form widget choice — are deferred to
the per-PR PLAN.md rather than pre-decided here.
