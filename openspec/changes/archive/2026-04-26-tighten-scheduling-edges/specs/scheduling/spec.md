## MODIFIED Requirements

### Requirement: Reject assignment of disabled users

The system SHALL reject an admin attempt to assign a disabled user with HTTP 409 and error code `USER_DISABLED`. The check SHALL be performed twice: once before the apply transaction (fast-fail for UX latency) and once again inside the transaction by re-reading `users.status` with `FOR UPDATE` after locking the user's schedule. The in-tx check is the correctness floor and ensures a user disabled between the pre-tx read and the apply commit is still rejected.

The same in-tx check SHALL be applied on the shift-change apply paths (`ApplySwap`, `ApplyGive`) for every user being mutated (receiver of give, both swap participants).

#### Scenario: Admin tries to assign a disabled user

- **GIVEN** a user whose account is disabled
- **WHEN** an admin creates an assignment with `user_id` set to that user
- **THEN** the response is HTTP 409 with error code `USER_DISABLED`

#### Scenario: User disabled after the request was created but before apply

- **GIVEN** a pending `give_direct` to user `U`, where `U` was active when the request was created
- **AND** an approve operation has already been authorized for `U`
- **WHEN** an admin disables `U` before the apply transaction's status check
- **THEN** the apply transaction's in-tx status check locks `U`'s user row and observes `U.status = disabled`
- **AND** the apply rolls back
- **AND** the response is HTTP 409 with error code `USER_DISABLED`

#### Scenario: User disabled between admin's pre-tx check and the insert

- **GIVEN** an admin calls `POST /publications/{id}/assignments` for user `U`
- **AND** the pre-tx user-status read shows `U.status = active`
- **AND** another admin disables `U` in the millisecond between the pre-tx read and the insert tx
- **WHEN** the insert transaction's in-tx status check runs
- **THEN** it observes `U.status = disabled`
- **AND** the insert rolls back
- **AND** the response is HTTP 409 with error code `USER_DISABLED`

### Requirement: Auto-assign replaces the full assignment set via MCMF

`POST /publications/{id}/auto-assign` SHALL run a min-cost max-flow solver over the candidate pool and SHALL replace the entire assignment set for the publication inside one transaction, so a partial result is never observed.

The candidate pool SHALL be derived by joining `availability_submissions` with the user's *current* `user_positions` and the user's *current* `users.status`. A submission whose `(user_id, position_id)` is no longer present in `user_positions`, or whose `user_id` is no longer `status = 'active'`, SHALL NOT contribute to the candidate pool, even though the submission row itself remains in the database (admin can re-add the position to restore it).

The graph SHALL be constructed as follows: a source `s`; for each user with at least one candidacy, per-weekday maximal overlap groups of slots the user submitted availability for (a user may take at most one slot per overlap group); up to `min(#groups, total_demand)` per-user "seat" nodes between `s` and a central "employee" node; one node per `(slot, position)` pair (i.e., per `template_slot_positions` row with candidacy); an intermediate `(user, slot)` node of capacity 1 between the user and any `(slot, position)` node for that slot (so a user can hold at most one position in the same slot, consistent with the `UNIQUE(publication_id, user_id, slot_id)` natural key); `(slot, position)` nodes connected to sink `t` with capacity `required_headcount` and a negative coverage bonus; all user-side edges of capacity 1; seat edges with costs that grow linearly with the seat index so work is spread across employees. The coverage bonus SHALL be large and negative (`-2 * total_demand`) so demand fill dominates spreading.

The solver SHALL NOT optimise for fairness over time, seniority, or preference weighting; those are out of scope. Admins MAY hand-edit any assignment afterward.

#### Scenario: Auto-assign is atomic

- **GIVEN** a publication with an existing assignment set
- **WHEN** an admin calls `POST /publications/{id}/auto-assign`
- **THEN** the response reflects the new assignment set with the previous set fully replaced, or an error with the previous set untouched — no partial replacement is observed

#### Scenario: Auto-assign does not double-book within an overlap group

- **GIVEN** a user who submitted availability for two slots that overlap on the same weekday
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those slots

#### Scenario: Auto-assign does not put a user in two positions of the same slot

- **GIVEN** a user who submitted availability for two positions within the same slot `S`
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those `(S, position)` pairs, consistent with the per-slot unique key

#### Scenario: Auto-assign skips submissions whose qualification was revoked

- **GIVEN** a user `U` who submitted availability for `(slot S, position P)` while qualified for `P`
- **AND** an admin removed `P` from `U`'s `user_positions` before auto-assign runs
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include `(U, S, P)`
- **AND** auto-assign does not assign `U` to `(S, P)`
- **AND** the `availability_submissions` row for `(U, S, P)` is unchanged in the database (it stays for potential future re-qualification)

#### Scenario: Auto-assign skips submissions from disabled users

- **GIVEN** a user `U` who submitted availability and was later disabled
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include any `(U, slot, position)` rows

### Requirement: Time-conflict check before applying

Before applying a swap or a give, the service SHALL recompute the receiver's full weekly assignment set as it would be after applying and SHALL reject with `SHIFT_CHANGE_TIME_CONFLICT` (HTTP 409) if any two assignments would share a weekday and overlap in time (using the overlap predicate `a.start < b.end && b.start < a.end` on the referenced slots' `start_time` and `end_time`).

The check SHALL be performed twice: once before the apply transaction (fast-fail for UX latency) and once again *inside* the apply transaction, after the transaction has acquired a per-(publication, user) transaction lock and row-level locks (`SELECT ... FOR UPDATE`) on every existing assignment row of every user being mutated in that publication. The in-tx check is the correctness floor and ensures concurrent shift-change applies cannot defeat the conflict check, including when the target user had zero existing assignment rows at the start of the transaction.

Understaffing SHALL NOT cause rejection at this step — it is acceptable for the receiver to take an assignment that leaves the original slot-position short-handed, because `required_headcount` is advisory.

#### Scenario: Overlap with existing weekly assignment rejects the apply

- **GIVEN** a pending `give_direct` whose acceptance would place the receiver in two slots that overlap on the same weekday
- **WHEN** the receiver accepts
- **THEN** the response is HTTP 409 with error code `SHIFT_CHANGE_TIME_CONFLICT`

#### Scenario: Leaving the origin slot-position understaffed does not block apply

- **GIVEN** the origin `(slot, position)` would fall below `required_headcount` after the give is applied
- **WHEN** the receiver accepts and no other rule is violated
- **THEN** the apply succeeds

#### Scenario: Concurrent shift-change apply cannot bypass the conflict check

- **GIVEN** two pending shift-change requests, R1 (give_pool) and R2 (give_direct), both targeting the same user `U`
- **AND** R1's added slot and R2's added slot overlap in time on the same weekday
- **AND** `U`'s pre-tx fast-fail check passes for both R1 and R2 (because at the moment each pre-tx check runs, the other has not committed)
- **WHEN** both apply transactions begin near-simultaneously
- **THEN** the first transaction to acquire `U`'s per-publication transaction lock commits successfully
- **AND** the second transaction blocks on that same per-publication user lock until the first commits
- **AND** the second transaction's in-tx conflict re-check observes the newly-committed assignment from the first
- **AND** the second transaction returns HTTP 409 `SHIFT_CHANGE_TIME_CONFLICT`
- **AND** `U` does NOT end up holding both overlapping assignments

### Requirement: Admin CreateAssignment rejects same-weekday slot overlap

`POST /publications/{id}/assignments` SHALL, after the state, qualification, and disabled-user gates, recompute the target user's existing assignments in the same publication and SHALL reject with `ASSIGNMENT_TIME_CONFLICT` (HTTP 409) if the new assignment's slot would overlap in time with any existing same-weekday slot the user already holds (overlap predicate: `a.start < b.end && b.start < a.end`). The check SHALL use the referenced slots' `start_time` and `end_time`.

The check SHALL be performed twice: once before the insert transaction (fast-fail for UX latency) and once again *inside* the insert transaction, after the transaction has acquired a per-(publication, user) transaction lock and row-level locks on every existing assignment row of the target user in that publication. The in-tx check is the correctness floor and ensures concurrent shift-change applies (or a concurrent admin assignment for the same user) cannot defeat the overlap check, including when the target user had zero existing assignment rows at the start of the transaction.

Understaffing SHALL NOT cause rejection at this step.

#### Scenario: Overlap with existing weekly assignment rejects the create

- **GIVEN** user `U` already assigned to `Mon 09:00-11:00 / position P1`
- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id: U, slot_id: S', position_id: P2 }` where slot `S'` is `Mon 10:00-12:00`
- **THEN** the response is HTTP 409 with error code `ASSIGNMENT_TIME_CONFLICT`
- **AND** no assignment row is written
- **AND** no `assignment.create` audit event is recorded

#### Scenario: Touching boundaries do not count as overlap

- **GIVEN** user `U` already assigned to `Mon 09:00-10:00 / position P1`
- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id: U, slot_id: S', position_id: P2 }` where slot `S'` is `Mon 10:00-12:00`
- **THEN** the request succeeds (boundaries touch but do not overlap)

#### Scenario: Overlap across different weekdays is not flagged

- **GIVEN** user `U` already assigned to `Mon 09:00-11:00`
- **WHEN** an admin creates a `Tue 09:00-11:00` assignment for the same user
- **THEN** the request succeeds (different weekday)

#### Scenario: Concurrent shift-change cannot defeat admin's overlap check

- **GIVEN** an admin's `POST /publications/{id}/assignments` for user `U` with slot `S_new` (Mon 10:00-12:00)
- **AND** the pre-tx fast-fail check passes (U has no Mon assignment yet)
- **AND** concurrently, a shift-change apply commits, giving `U` slot `S_other` (Mon 11:00-13:00)
- **WHEN** the admin's insert transaction reaches the in-tx overlap re-check
- **THEN** the re-check, run after `SELECT ... FOR UPDATE` on `U`'s assignments, observes `S_other` and detects the overlap with `S_new`
- **AND** the insert rolls back
- **AND** the response is HTTP 409 with error code `ASSIGNMENT_TIME_CONFLICT`
