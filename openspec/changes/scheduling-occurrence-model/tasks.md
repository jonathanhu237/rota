## 1. Database migration

- [ ] 1.1 Add migration `migrations/00011_scheduling_occurrence_model.sql` (or next sequential number) with the goose Up and Down blocks specified in `design.md` decision D-11. Up adds `publications.planned_active_until`, drops `publications.ended_at`, replaces the publication window CHECK, creates `assignment_overrides`, and adds `shift_change_requests.occurrence_date`. Down reverses every step. Verify: `make migrate-down && make migrate-up && make migrate-down` runs clean against a local Postgres; CI's `migrations-roundtrip` job exercises the same path.

## 2. Backend model layer

- [ ] 2.1 Update `backend/internal/model/publication.go`: replace `EndedAt *time.Time` with `PlannedActiveUntil time.Time` (non-pointer, required). Adjust JSON tags. Verify: `cd backend && go build ./...`.
- [ ] 2.2 Update `model.Publication.ResolveEffectiveState` (or wherever effective-state resolution lives) to implement the cascade in spec requirement *Effective state resolution on read*: ENDED stored → ENDED; ACTIVE+now>until → ENDED; PUBLISHED/ACTIVE → stored; time-driven COLLECTING/ASSIGNING/DRAFT as today. Verify: existing model unit tests adapted; new tests cover the ENDED-by-clock case.
- [ ] 2.3 Add `backend/internal/model/assignment_override.go` with the `AssignmentOverride` struct: `ID, AssignmentID, OccurrenceDate (time.Time as DATE — use `civil.Date` or `time.Time` with date-only normalization, follow whatever pattern repo uses for DATE), UserID, CreatedAt`. Verify: `go build ./...`.
- [ ] 2.4 Update `model.ShiftChangeRequest`: add `OccurrenceDate time.Time` and `CounterpartOccurrenceDate *time.Time`. Update marshaling. Verify: `go build ./...`.
- [ ] 2.5 Add `IsValidOccurrence(pub, slot, occurrenceDate, now) error` helper in the model or a scheduling util package. Returns specific error sentinels for: weekday mismatch, outside window, in-past. Verify: unit test covers all three rejection paths and the success path.

## 3. Backend repository layer

- [ ] 3.1 Update `backend/internal/repository/publication.go`: SELECT/INSERT/UPDATE include `planned_active_until` and exclude `ended_at`. Add `UpdatePublicationFields` (or extend existing update) to support the PATCH path with a partial-field update. Verify: integration tests that round-trip a publication compile and pass with the new column.
- [ ] 3.2 Add `backend/internal/repository/assignment_override.go`: `Insert`, `DeleteByAssignment` (used by cascade), `ListForPublicationWeek(publicationID, weekStart)` returning override rows joined with their baseline assignment metadata for roster construction. Verify: integration tests for each method (`go test -tags=integration ./internal/repository -run AssignmentOverride`).
- [ ] 3.3 Update `backend/internal/repository/shift_change.go`: SELECT/INSERT include `occurrence_date` and `counterpart_occurrence_date`. The existing apply repository helper is rewritten in §4; this task just covers data-layer reads/writes of the new columns. Verify: `go build ./...` and existing repository tests adapted.
- [ ] 3.4 Update the roster query (`repository/publication_roster.go` or the equivalent) to LEFT JOIN `assignment_overrides` on `(assignment_id, occurrence_date)` for the requested week, returning the override's `user_id` when present and the baseline `assignments.user_id` otherwise. Verify: integration test asserts override takes precedence; baseline used when no override.
- [ ] 3.5 Update `assignments` deletion path so the existing cascade handler also surfaces the count of overrides removed by `ON DELETE CASCADE` (informational; no behavior change beyond the FK). Verify: integration test deletes an assignment with two overrides and confirms both are gone.

## 4. Backend service layer — publications

- [ ] 4.1 Add `PublicationService.UpdatePublication(ctx, id, params UpdatePublicationParams)` that validates window invariant (`planned_active_from < new_until`), audits the change, and updates only the supplied fields (`name`, `description`, `planned_active_until`). Reject with `ErrInvalidPublicationWindow` if the new window is invalid. Verify: unit test covers success, invalid window, and partial-field update.
- [ ] 4.2 Reroute `PublicationService.EndPublication` (called by `POST /publications/{id}/end`) through `UpdatePublication` with `planned_active_until = clock.Now()`. Reject with `ErrPublicationNotActive` if the publication's effective state is not `ACTIVE`. Verify: unit tests for ACTIVE → ENDED-via-end, and rejection in non-ACTIVE states.
- [ ] 4.3 Add the on-create sweep in `PublicationService.CreatePublication`: in the same transaction, run `UPDATE publications SET state='ENDED' WHERE state='ACTIVE' AND planned_active_until <= NOW()` before the `INSERT`. Verify: integration test creates publication A with `until` in the past, then creates publication B; both succeed and A is in stored state ENDED post-create.
- [ ] 4.4 Update `PublicationService.CreatePublication` to require and persist `planned_active_until`. Verify: handler unit tests (§6) cover the new required field.

## 5. Backend service layer — shift-change apply

- [ ] 5.1 Rewrite `ShiftChangeService.ApplyGive` to write an `assignment_overrides` row instead of mutating `assignments.user_id`. The transaction: re-read the request and the baseline assignment with `FOR UPDATE`; verify the captured `(assignment_id, publication_id, user_id)` still matches; verify the receiver's `users.status = 'active'` with `FOR UPDATE`; insert the override; transition the request to `approved`. Verify: unit and integration tests for `give_direct` and `give_pool` happy paths.
- [ ] 5.2 Rewrite `ShiftChangeService.ApplySwap` to write two `assignment_overrides` rows (requester's slot+date → counterpart user; counterpart's slot+date → requester). Same in-tx checks for both users' status. Verify: unit and integration tests for swap happy path and cross-occurrence swaps (different dates).
- [ ] 5.3 Update `ShiftChangeService.CreateRequest` to require and validate `occurrence_date` (and `counterpart_occurrence_date` for swaps) via `IsValidOccurrence`. Reject with `INVALID_OCCURRENCE_DATE` on validation failure. Compute and store `expires_at` per design D-6 (occurrence start time). Verify: unit tests for create success, weekday-mismatch reject, out-of-window reject, in-past reject.
- [ ] 5.4 Update the cascade-invalidate logic in `ShiftChangeService` (called from `AssignmentService.Delete`) to span every pending request that references the deleted assignment, regardless of `occurrence_date`. Each invalidated request emits an audit event and an email as today. Verify: integration test deletes an assignment with three pending requests on three different occurrence_dates and asserts all three are invalidated.

## 6. Backend handler layer

- [ ] 6.1 Add `PATCH /publications/{id}` handler. Body: any subset of `{ name, description, planned_active_until }`. Calls `UpdatePublication`. Verify: handler tests for success, invalid window, missing publication.
- [ ] 6.2 Update `POST /publications` handler to accept and pass through `planned_active_until`. Verify: tests cover the new required field; missing field returns `INVALID_REQUEST`.
- [ ] 6.3 Update `POST /publications/{id}/shift-changes` handler: body now requires `occurrence_date`; for `swap` also requires `counterpart_occurrence_date`. Verify: handler tests for create success, missing occurrence_date, swap missing counterpart date.
- [ ] 6.4 Update `GET /publications/{id}/roster`: accept `?week=YYYY-MM-DD`; default to the current-week-or-first-week as per spec. Reject Tuesday-or-out-of-window with `INVALID_OCCURRENCE_DATE`. Verify: handler tests for explicit-week, default-week, and bad-week.
- [ ] 6.5 Confirm `POST /publications/{id}/end` still returns 204 / state-aware errors via the new `EndPublication` path. Verify: existing handler test continues to pass with the rewired internals.

## 7. Frontend

- [ ] 7.1 Update the shift-change creation form to include an "occurrence" picker: dropdown listing every valid `occurrence_date` for the requester's selected assignment in the current publication, filtered to occurrences whose actual start time is `> NOW()`. For `swap`, also include a counterpart-occurrence picker tied to the chosen counterpart's assignment. Verify: lint + unit tests pass; manual smoke test confirms picker displays expected dates.
- [ ] 7.2 Surface `occurrence_date` in the shift-change list and detail views (next to slot day/time). Verify: same.
- [ ] 7.3 Update the publication admin view to show and edit `planned_active_until`. Hooks into the new `PATCH` endpoint. Verify: lint + unit tests pass; manual smoke test confirms editing the field updates the row and adjusts effective state.
- [ ] 7.4 Update the roster view to support week-by-week navigation: arrows that step `?week` parameter forward/back; default landing on the current-or-first week. Verify: same.
- [ ] 7.5 Update `frontend/src/lib/api-error.ts` (or wherever client-side error mapping lives) to map `INVALID_OCCURRENCE_DATE` to a user-friendly message. Verify: existing API-error tests adapted.

## 8. Seed updates

- [ ] 8.1 Update `backend/cmd/seed/scenarios/full.go` and `stress.go` to set `planned_active_until` on every publication insert. Pick a sane default (e.g., `planned_active_from + 8 weeks` for `full`; varied across the four publications in `stress` to give the UI realistic week counts). Verify: `make seed`, `make seed SCENARIO=full`, `make seed SCENARIO=stress` all run end-to-end; `psql -c "SELECT name, planned_active_from, planned_active_until FROM publications"` shows reasonable windows.

## 9. Documentation

- [ ] 9.1 Update `README.md` Environment Variables table (no new var) and the brief "Seeding Dev Data" / publication explanation if it mentions `ended_at`. Verify: render locally and re-read.
- [ ] 9.2 Update inline service-layer comments where they describe assignment mutation on apply — they will lie after this change. Replace with one-line notes pointing at `assignment_overrides`. Verify: code review.

## 10. Final verification

- [ ] 10.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0.
- [ ] 10.2 Frontend clean: `cd frontend && pnpm lint && pnpm test && pnpm build` — every step exits 0.
- [ ] 10.3 Migrations round-trip clean: `make migrate-down && make migrate-up && make migrate-down && make migrate-up`.
- [ ] 10.4 Smoke test (manual):
    - (a) `make migrate-down && make migrate-up && make seed SCENARIO=full`. Login as `admin@example.com` / `pa55word`. Auto-assign the publication. Patch its `planned_active_until` shorter then longer; confirm effective state changes accordingly.
    - (b) As an employee, create a `give_pool` for a future occurrence; as another employee, claim it; observe that the override appears on the roster only for that week, baseline preserved on other weeks.
    - (c) Create a swap between two employees on different occurrence_dates; verify both overrides land.
    - (d) Admin deletes the original baseline assignment; confirm the pending request is invalidated and the override row is gone.
- [ ] 10.5 Confirm CI is green on the `change/scheduling-occurrence-model` branch: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`.
