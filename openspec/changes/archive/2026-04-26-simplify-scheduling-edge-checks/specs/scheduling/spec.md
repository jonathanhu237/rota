## REMOVED Requirements

### Requirement: Time-conflict check before applying

**Reason**: The `template_slots_no_overlap_excl` GIST exclusion constraint (shipped in `refactor-to-slot-position-model`) makes overlapping assignments structurally impossible within a publication. Every assignment references a slot, and all slots in a publication share one locked template. The GIST forbids any two slots in the same `(template_id, weekday)` from having overlapping time ranges. Therefore no two assignments in the same publication can have overlapping slot times. The service-layer pre-tx and in-tx time-conflict checks this requirement described were redundant — they could not return a violation against any database state the GIST allowed. The redundant code (`ensureSwapFitsSchedule`, `ensureGiveFitsSchedule`, the in-tx conflict logic in `LockAndCheckUserSchedule`) is removed alongside this requirement.

**Migration**: Callers that previously expected `SHIFT_CHANGE_TIME_CONFLICT` from a swap or give apply path will no longer observe it — but the underlying invariant is still enforced. Admins cannot create overlapping slots in the first place (see "Template, slot, and slot-position data model" → "Overlapping slots in the same template and weekday are rejected at the database"). Existing frontend i18n entries for `SHIFT_CHANGE_TIME_CONFLICT` are inert but harmless and may be cleaned up by a future i18n audit.

### Requirement: Admin CreateAssignment rejects same-weekday slot overlap

**Reason**: Same as above. Under the GIST exclusion, the slots required to construct an overlapping assignment cannot coexist in `template_slots`; an admin cannot create an assignment whose slot overlaps another existing-assignment's slot for the same user. The pre-tx and in-tx overlap checks in `CreateAssignment` (`hasAssignmentTimeConflict` and the in-tx variant) are redundant under this constraint and removed.

**Migration**: Admins who define non-overlapping slots and then assign users to them experience no behavioral change. Admins who attempt to define overlapping slots are now rejected at slot creation (HTTP 409 `TEMPLATE_SLOT_OVERLAP`) rather than at assignment creation; this is a clearer point of failure and matches the data-model boundary. Existing frontend i18n entries for `ASSIGNMENT_TIME_CONFLICT` are inert but harmless.

## MODIFIED Requirements

### Requirement: Reject assignment of disabled users

The system SHALL reject an admin attempt to assign a disabled user with HTTP 409 and error code `USER_DISABLED`. The check SHALL be performed twice: once before the apply transaction (fast-fail for UX latency) and once again inside the transaction by re-reading `users.status` with `FOR UPDATE`. The in-tx check is the correctness floor and ensures a user disabled between the pre-tx read and the apply commit is still rejected.

The same in-tx check SHALL be applied on the shift-change apply paths (`ApplySwap`, `ApplyGive`) for every user being mutated (receiver of give, both swap participants).

#### Scenario: Admin tries to assign a disabled user

- **GIVEN** a user whose account is disabled
- **WHEN** an admin creates an assignment with `user_id` set to that user
- **THEN** the response is HTTP 409 with error code `USER_DISABLED`

#### Scenario: User disabled after the request was created but before apply

- **GIVEN** a pending `give_direct` to user `U`, where `U` was active when the request was created
- **AND** an approve operation has already been authorized for `U`
- **WHEN** an admin disables `U` before the apply transaction's status check
- **THEN** the apply transaction's in-tx status check `SELECT status FROM users WHERE id = $userID FOR UPDATE` observes `U.status = disabled`
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
