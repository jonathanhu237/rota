## 1. Schema migration

- [x] 1.1 Author `migrations/00016_decouple_weekday_from_slot.sql` with the `+goose Up` + `+goose Down` blocks per design D-3, including: TRUNCATE of `leaves`, `shift_change_requests`, `assignment_overrides`, `assignments`, `availability_submissions`, `template_slot_positions`, `template_slots`; creation of `template_slot_weekdays`; drop of `template_slots.weekday` + GIST + `(template_id, weekday, start_time)` index; add `(template_id, start_time)` index; add `weekday INTEGER NOT NULL CHECK BETWEEN 1 AND 7` to `availability_submissions` and `assignments`; widen their UNIQUEs; install composite FKs to `template_slot_weekdays`; install the overlap trigger function with SQLSTATE `23P01`; emit a `RAISE NOTICE` re-seed hint. Verify by `make migrate-up && make migrate-down 1 && make migrate-up`.
- [x] 1.2 Verify `make migrate-status` reports clean and that re-applying yields no error. Verify by `make migrate-status`.

## 2. Backend model + repository

- [x] 2.1 Update `backend/internal/model/template.go`: drop `Weekday int` from `TemplateSlot`, add `Weekdays []int`; add `TemplateSlotWeekday` struct if needed for repo internals.
- [x] 2.2 Update `backend/internal/model/availability_submission.go`: add `Weekday int`.
- [x] 2.3 Update `backend/internal/model/assignment.go`: add `Weekday int`.
- [x] 2.4 Update `backend/internal/model/publication.go`: rename/refactor `IsValidOccurrence(publication, slot, occurrence_date)` to take the slot's weekday set or split into `IsValidOccurrence(publication, slot, weekday, occurrence_date)` + `IsValidOccurrenceForAssignment(publication, assignment, occurrence_date)` per design D-6. Update `weekdayToSlotValue` callers.
- [x] 2.5 Update repository SQL in `backend/internal/repository/template_repo*.go`: slot CRUD now writes to `template_slot_weekdays` alongside `template_slots`; reads return `Weekdays []int` aggregated via `array_agg(weekday ORDER BY weekday)`; sort order is `(start_time, end_time, id)`. Do not enforce uniqueness on `(template_id, start_time, end_time)`; same-time disjoint-weekday slots are valid. Translate trigger SQLSTATE `23P01` to `ErrTemplateSlotOverlap` (no change to handler; it already maps the sentinel).
- [x] 2.6 Update `backend/internal/repository/availability_*.go`: insert/delete carries `weekday`; queries return `weekday`.
- [x] 2.7 Update `backend/internal/repository/assignment_*.go`: insert/delete/list carry `weekday`; uniqueness and `assignments_position_belongs_to_slot` trigger paths unchanged.
- [x] 2.8 Update `backend/internal/repository/shift_change_*.go` and `leave_*.go`: any read of `slot.Weekday` becomes `assignment.Weekday`. The `expires_at` insert uses `assignment.Weekday`.
- [x] 2.9 Verify by `cd backend && go build ./...`.

## 3. Backend service layer

- [x] 3.1 Update `backend/internal/service/template.go`: slot create/update validate `weekdays` (non-empty, in `[1,7]`, deduplicated), atomic replace on patch.
- [x] 3.2 Update `backend/internal/service/availability.go`: submission create/delete carries `weekday`. Qualification check unchanged.
- [x] 3.3 Update `backend/internal/service/autoassign.go`: candidate-pool query joins on `(slot_id, weekday)`; per-weekday overlap groups derive from `submission.weekday`; the `slotPosition`-equivalent struct gains `Weekday`. The MCMF graph topology is unchanged.
- [x] 3.4 Update `backend/internal/service/assignment.go`: create/delete carries `weekday`; admin board read groups by `(slot_id, weekday, position_id)`.
- [x] 3.5 Update `backend/internal/service/shift_change.go`: `expires_at` derivation reads `assignment.Weekday`; occurrence validation uses `IsValidOccurrenceForAssignment` per D-6.
- [x] 3.6 Update `backend/internal/service/leave.go`: same — read weekday from the assignment, not the slot.
- [x] 3.7 Update `backend/internal/service/publication_pr*.go`: roster cell key, sort, and serialization use `(weekday, start_time)` from the assignment row.
- [x] 3.8 Verify by `cd backend && go build ./... && go vet ./...`.

## 4. Backend handler layer

- [x] 4.1 Update `backend/internal/handler/template.go`: `POST/PATCH /templates/{id}/slots` accepts `weekdays: int[]` (reject empty / out-of-range / non-int). Reject legacy `weekday: int`. Response shape carries `weekdays: int[]` per slot, sorted ascending.
- [x] 4.2 Update `backend/internal/handler/availability.go`: `POST` body accepts `{ slot_id, weekday }`; `DELETE` URL pattern is `/publications/{id}/submissions/{slot_id}/{weekday}`. `GET /publications/{id}/submissions/me` returns `[ {slot_id, weekday} ]`.
- [x] 4.3 Update `backend/internal/handler/assignment.go`: `POST` body accepts `{ user_id, slot_id, weekday, position_id }`; reject body missing `weekday` with HTTP 400 / `INVALID_REQUEST`; assignment-board cell key includes `weekday`.
- [x] 4.4 Update `backend/internal/handler/shift_change.go` and `leave.go`: occurrence validation calls the new helper signatures; response shapes for shift-changes / leaves are unchanged on the wire (occurrence already carries weekday in the response).
- [x] 4.5 Update `backend/internal/handler/roster.go`: response cell ferries `weekday` from the assignment column.
- [x] 4.6 Verify by `cd backend && go build ./...`.

## 5. Backend tests

- [x] 5.1 Update integration test fixture builders in `backend/internal/service/autoassign_integration_test.go`: replace per-weekday `insertTemplateSlot(template, weekday, start, end)` calls with `insertTemplateSlot(template, []int{weekday, ...}, start, end)`. Existing test names and assertions stay; only the construction path changes. Approximately 50 call sites — keep mechanical.
- [x] 5.2 Update other `*_integration_test.go` files with the same pattern: `availability_integration_test.go`, `assignment_integration_test.go`, `shift_change_integration_test.go`, `leave_integration_test.go`, `publication_pr*_test.go` — anywhere a fixture inserts a slot + composition.
- [x] 5.3 Update unit tests (`*_test.go`) with mocked repositories: any mock that returns `TemplateSlot{Weekday: 1}` becomes `TemplateSlot{Weekdays: []int{1}}`; submissions/assignments mocks add `Weekday`.
- [x] 5.4 Add new scenarios from the spec delta where they expose behavior not yet covered: "two slots with same time range but disjoint weekdays coexist", "removing a weekday cascades to referencing submissions and assignments", "submission for a weekday not in the slot's set is rejected", "assignment for a weekday not in the slot's set is rejected".
- [x] 5.5 Verify by `cd backend && go test ./...` (unit) and `cd backend && go test -tags=integration ./...` (integration, with Postgres up).

## 6. Seed scenarios

- [x] 6.1 Update `backend/cmd/seed/scenarios/common.go`: any helper that today inserts a slot row per weekday (e.g., `insertSlots`) now inserts one slot row + N `template_slot_weekdays` rows; `template_slot_positions` insertion happens once per logical slot (not per weekday).
- [x] 6.2 Update `backend/cmd/seed/scenarios/full.go`: `fullSlotDefinitions()` shrinks from ~10 entries to logical-slot-with-weekday-set entries (probably 6-7). Submissions seeding helper iterates `(slot, weekday)` pairs.
- [x] 6.3 Update `backend/cmd/seed/scenarios/stress.go`: similarly coalesce same-time-different-weekday entries.
- [x] 6.4 Update `openspec/changes/archive/2026-04-27-seed-realistic-data/generate_realistic_seed.py` is archived; create a new `backend/cmd/seed/scenarios/realistic_gen.py` (or update the generator that lives alongside `realistic.go` after archive) so it emits the new shape: 5 logical slots, 35 `template_slot_weekdays` rows (5 × 7), 10 `template_slot_positions` rows (4 daytime × 2 + 1 evening × 2), per-`(slot, weekday)` availability submissions. Re-run the generator and commit the regenerated `realistic.go`.
- [x] 6.5 Verify each scenario by `cd /Users/jonathanhu237/code/rota && make seed SCENARIO=basic && make seed SCENARIO=full && make seed SCENARIO=stress && make seed SCENARIO=realistic` with local Postgres up. Smoke-check row counts: `template_slots`, `template_slot_weekdays`, `template_slot_positions`, `availability_submissions`, `assignments`.

## 7. Frontend

- [x] 7.1 Update Zod schemas under `frontend/src/api/templates*.ts` (or equivalent): slot type carries `weekdays: number[]` instead of `weekday: number`. Sort key `(start_time, end_time, id)`.
- [x] 7.2 Update Zod schemas under `frontend/src/api/availability*.ts`: submission requests/responses carry `weekday`. URL builder for the DELETE includes the weekday segment.
- [x] 7.3 Update Zod schemas under `frontend/src/api/assignments*.ts`: assignment row + create body carry `weekday`; assignment-board cell key includes `weekday`.
- [x] 7.4 Rewrite `frontend/src/routes/templates.$id.tsx` (or the file that owns the template-detail page) per design D-7: flat slot list ordered by `(start_time, end_time)`; each slot row shows the composition once and a 7-cell Mon-Sun chip strip for its weekday set. Locked-template state hides per-slot edit/delete buttons. Drop the `星期一 / 星期二 …` accordion.
- [x] 7.5 Update the availability page (`frontend/src/routes/availability/...`): each cell of the (weekday × time) grid maps to `(slot_id, weekday)`; submission tick/un-tick uses the new POST body and DELETE URL.
- [x] 7.6 Update the assignment-board page: cells keyed on `(slot_id, weekday, position_id)`; create-assignment dialog includes weekday (if not already passed by context).
- [x] 7.7 Update the roster page and shift-change UI: read weekday from the assignment row in the response; no visual change.
- [x] 7.8 Update i18n strings under `frontend/src/locales/{en,zh}/...` if any string mentions "slot weekday" — none should, but verify.
- [x] 7.9 Verify by `cd frontend && pnpm lint && pnpm test && pnpm build`.

## 8. End-to-end smoke

- [x] 8.1 With local Postgres + backend + frontend running, walk the realistic scenario in the browser: log in as admin, view `/templates/1`, confirm the new layout (flat slot list, weekday chips, single composition per slot). Confirm a locked template hides edit affordances.
- [x] 8.2 Trigger auto-assign on the realistic publication, confirm it produces an assignment set whose row count is consistent with `(slot, weekday, position)` cells × `required_headcount`.
- [x] 8.3 As an employee, view the availability page and confirm submissions persist and reload correctly.
- [x] 8.4 As an admin, drag-drop on the assignment board and confirm the new `(slot, weekday)` cell key flows through unchanged.

## 9. Spec sync

- [x] 9.1 Confirm the change-folder spec deltas at `openspec/changes/decouple-weekday-from-slot/specs/{scheduling,dev-tooling}/spec.md` match the implemented behavior. Do not edit `openspec/specs/*/spec.md` directly — `/opsx:archive` will sync them.

## 10. Final gates

- [x] 10.1 Run the full backend gate from the project root: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...`. All clean.
- [x] 10.2 Run the full frontend gate: `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 10.3 Run `openspec validate decouple-weekday-from-slot --strict` and confirm clean.
