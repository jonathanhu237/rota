## 1. Database migration

- [x] 1.1 Add `migrations/000NN_drop_position_from_availability_submissions.sql` (next sequential, likely `00015`) per design D-3. Up drops the composite FK on `(slot_id, position_id)`, drops the existing 4-column unique index, drops the `position_id` column, recreates the 3-column unique constraint and the `(publication_id, slot_id)` read index. Down recreates the column as nullable, restores the FK and unique-key shape (any existing rows will carry NULL for `position_id` — acceptable per project's no-production-data stance). Verify: `make migrate-down && make migrate-up && make migrate-down` clean; CI's `migrations-roundtrip` exercises Up/Down.

## 2. Backend model + repository

- [x] 2.1 In `backend/internal/model/publication.go` (or wherever `AvailabilitySubmission` is defined), drop the `PositionID` field from the struct. Verify: `cd backend && go build ./...` — expect compile failures cascading to every leftover reference; resolve them in subsequent tasks.

- [x] 2.2 Update `backend/internal/repository/publication.go` (or whichever file holds the submission queries):
    - INSERT loses `position_id`; the conflict path (UNIQUE violation) still maps to `ErrAvailabilitySubmissionExists` (or whichever sentinel is used).
    - DELETE drops the `position_id` predicate.
    - Any SELECT projecting `position_id` drops the column. Where the previous SELECT joined to `template_slot_positions` to verify the `(slot_id, position_id)` composite, replace with a check that `slot_id` belongs to the publication's template (the existing FK to `template_slots` already enforces row-level integrity; the qualification overlap check moves to the service layer).
    - Add a helper `ListUserPositionsForSlot(ctx, slotID)` if needed by the service layer's qualification check.
  Verify: `go build ./...` and existing repository unit tests pass after their fixture updates.

- [x] 2.3 Update `backend/internal/repository/publication_db_test.go` (or the integration tests for submissions) to drop `PositionID` from test fixtures and assert the new shape. Verify: `go test -tags=integration ./internal/repository -run AvailabilitySubmission -count=1`.

## 3. Backend service layer

- [x] 3.1 In `backend/internal/service/publication.go`:
    - `CreateAvailabilitySubmissionInput` drops `PositionID`.
    - `DeleteAvailabilitySubmissionInput` drops `PositionID`.
    - `CreateAvailabilitySubmission` performs the new qualification check: load `user_positions(user_id)` and `template_slot_positions(slot_id)`; if the intersection is empty, return `ErrNotQualified` (HTTP 403 NOT_QUALIFIED at the handler).
    - The audit metadata for `availability.create` / `availability.delete` adjusts to record `slot_id` only (no `position_id`).
  Verify: `go build ./...` and `go test ./internal/service -run Availability -count=1`.

- [x] 3.2 Update `backend/internal/service/publication.go` auto-assign candidate-pool builder per design D-2 / D-6:
    - Build `viableCandidates = (user, slot)` rows where `submissions(user, slot)` exists, `user_positions(user) ∩ composition(slot) ≠ ∅`, and `users.status(user) = 'active'`.
    - In the MCMF graph, the `(user, slot)` intermediate node fans out to `(slot, position)` cells only for positions in `user_positions(user)`.
    - Update any documentation comments that explained the old "submission references a specific position" semantics.
  Verify: `go test ./internal/service -run AutoAssign -count=1` plus the integration test, which exercises a senior-qualified-for-multi-position scenario.

- [x] 3.3 In `backend/internal/service/publication.go`, update the assignment-board candidate derivation per design D-7 risk note: `candidates(slot, position)` now becomes "users who submitted availability for `slot` AND have `position` in their `user_positions`". The wire shape on `assignment-board` is unchanged. Verify: `go test ./internal/service -run AssignmentBoard -count=1`.

## 4. Backend handler

- [x] 4.1 Update `backend/internal/handler/publication.go`:
    - `createSubmissionRequest` drops `PositionID`; the body now only requires `slot_id`. (Per the existing convention of rejecting unknown fields established by the `template_shift_id` cleanup, surplus fields like `position_id` SHOULD be ignored quietly per design D-4 to avoid breaking clients mid-flight.)
    - `DeleteSubmission` route updates: previously `/submissions/{slot_id}/{position_id}`, now `/submissions/{slot_id}`. Update route registration and the handler function (read one path param).
    - `GET /publications/{id}/shifts/me` response uses the new shape with `composition` array; the handler shapes the response by joining `template_slots` with `template_slot_positions` and `positions` to produce the per-slot composition payload.
    - `GET /publications/{id}/submissions/me` returns `[ {slot_id} ]`.
  Verify: `go build ./...` and `go test ./internal/handler -run Availability -count=1`.

- [x] 4.2 Update handler tests to send the new request bodies / assert the new URL / response shapes; remove the regression-guard test that asserts unknown body fields are rejected (we now ignore stray `position_id`). Verify: same.

## 5. Frontend type + query layer

- [x] 5.1 In `frontend/src/lib/types.ts`:
    - The per-slot type used by the availability flow (today probably `QualifiedShift`) drops `position_id` and `required_headcount` at the top level.
    - Add a `composition: { position_id, position_name, required_headcount }[]` field.
  Verify: `pnpm build` — expect compile failures cascading; resolve them in subsequent tasks.

- [x] 5.2 In `frontend/src/lib/queries.ts`:
    - The `submissions` mutation's request payload shrinks to `{ slot_id }`.
    - The DELETE-submission mutation URL drops the trailing `/{position_id}` segment.
    - The `shifts/me` query response shape updates to match D-4's payload.
  Verify: `pnpm test` for `queries.test.ts`.

## 6. Frontend availability grid

- [x] 6.1 In `frontend/src/components/availability/availability-grid.tsx`:
    - The selection set tracks `slot_id` only (was `${slot_id}:${position_id}`).
    - `groupedShifts` renders one checkbox per slot per weekday.
    - Each slot's caption renders the `composition` array as `"前台负责人 × 1 / 前台助理 × 2"` (use i18n template — see 6.3).
    - `onToggle` signature collapses to `(slotID, checked)`.
  Verify: `pnpm test` for `availability-grid.test.tsx` (test case rewrites included).

- [x] 6.2 In `frontend/src/components/templates/group-qualified-shifts.ts`, the helper now groups by slot (one entry per slot per weekday with composition attached), not by `(slot, position)`. Verify: `pnpm test` for `group-qualified-shifts.test.ts`.

- [x] 6.3 Update `frontend/src/i18n/locales/en.json` and `zh.json`:
    - Replace `availability.shift.summary` (today shows `position` + `headcount` for one position) with a per-slot composition template, e.g., `availability.shift.composition: "{{summary}}"` plus a small per-position helper string like `availability.shift.compositionEntry: "{{position}} × {{count}}"`.
    - Remove now-unused i18n keys.
  Verify: `pnpm lint` finds no unused keys.

## 7. Spec & docs

- [x] 7.1 Spec delta is in this change folder. Archive sync at the end of the cycle pushes 5 modified `scheduling` requirements into `openspec/specs/scheduling/spec.md`. No direct main-spec edit during apply (don't repeat the drop-redis pattern that needed `--skip-specs`).

- [x] 7.2 Skim `README.md` for any user-facing copy describing "tick a position" or similar; update or delete if present. Verify: `grep -nE "position" README.md` shows nothing about availability submissions.

## 8. Final verification

- [x] 8.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0.
- [x] 8.2 Frontend clean: `cd frontend && pnpm lint && pnpm test && pnpm build` — every step exits 0.
- [x] 8.3 Migrations roundtrip: `make migrate-down && make migrate-up && make migrate-down && make migrate-up` clean.
- [x] 8.4 Smoke test (manual):
    - (a) `make migrate-down && make migrate-up && make seed SCENARIO=full && make run-backend && make run-frontend`.
    - (b) Login as `employee1@example.com`. The availability page now shows ONE checkbox per slot per weekday, with a composition caption underneath the time range. (Pre-change, multi-qualified users saw two checkboxes per slot.)
    - (c) Tick a slot. DevTools Network: POST body is `{ "slot_id": <int> }` (no `position_id`).
    - (d) Untick the same slot. DevTools Network: DELETE URL is `/api/publications/.../submissions/{slot_id}` (no trailing `/<position_id>`).
    - (e) Try ticking a slot whose composition is entirely outside your `user_positions` (the UI should not show this slot at all because `shifts/me` filters it out — verify by checking the API response in the Network tab).
    - (f) Login as admin, run auto-assign. Verify the assignment set covers the `(slot, position)` cells correctly: a multi-qualified user submitting for a single slot should be auto-routed to the cell that helps coverage most (the new behavior under D-2).
- [ ] 8.5 Confirm CI is green on `change/simplify-availability-submission`: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`.
