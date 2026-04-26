## 1. Backend model removal

- [ ] 1.1 Delete `backend/internal/model/template_shift.go`. Add `backend/internal/model/qualified_shift.go` defining `QualifiedShift` with fields `SlotID, PositionID, Weekday (int), StartTime (string), EndTime (string), RequiredHeadcount (int)`. Verify: `cd backend && go build ./...` — expect mass compile failures pointing at every leftover `model.TemplateShift` site.
- [ ] 1.2 Delete `model.ErrTemplateShiftNotFound`. All references SHALL switch to `model.ErrTemplateSlotPositionNotFound` (already defined). Verify: `go build ./...`.

## 2. Backend repository cleanup

- [ ] 2.1 Delete `backend/internal/repository/template_shift_compat.go` entirely.
- [ ] 2.2 In `backend/internal/repository/assignment.go`:
    - Remove the `TemplateShiftID` field from any `CreateAssignmentParams` / `UpdateAssignmentParams` / similar structs.
    - Simplify `resolveAssignmentRef` to take only `slotID, positionID` and return `(slotID, positionID, entryID, err)` — drop the `templateShiftID` branch entirely. The function still calls `getTemplateSlotPositionEntryID` to resolve the entry id where the trigger needs it.
    - Delete `getTemplateSlotPositionPairByEntryID` (no callers after the switch).
  Verify: `go build ./...` and `go test ./internal/repository -count=1`.
- [ ] 2.3 In `backend/internal/repository/publication.go`: rename method receivers / params that say `TemplateShift` to slot+position language. Likely targets: `ListQualifiedPublicationShifts` → `ListQualifiedPublicationSlotPositions`, plus query column aliases. Verify: same.
- [ ] 2.4 Update integration tests in `backend/internal/repository/*_test.go` that constructed `model.TemplateShift` rows or referenced the deleted fields. The tests should now construct rows via `template_slots` + `template_slot_positions` directly (the helpers should already exist post-refactor; if not, add them). Verify: `go test -tags=integration ./internal/repository -count=1`.

## 3. Backend service cleanup

- [ ] 3.1 In `backend/internal/service/publication.go` and `service/template.go`:
    - Rename any `ListQualifiedPublicationShifts` → `ListQualifiedPublicationSlotPositions`.
    - Update return types from `[]*model.TemplateShift` → `[]*model.QualifiedShift`.
    - Drop `service.ErrTemplateShiftNotFound` alias if present; fold into `ErrTemplateSlotPositionNotFound`.
  Verify: `go build ./...` and `go test ./internal/service -count=1`.
- [ ] 3.2 Sweep `backend/internal/service/*_test.go` for any `model.TemplateShift{}` literal or `TemplateShiftID` field references and update. Verify: same.

## 4. Backend handler updates

- [ ] 4.1 In `backend/internal/handler/publication.go`:
    - Remove `TemplateShiftID` field from `createSubmissionRequest` and `createAssignmentRequest`. Both bodies now require `{ slot_id, position_id }` (plus user_id for assignment).
    - Update the route registration / route handler for `DELETE /publications/{id}/submissions/{shift_id}` to `DELETE /publications/{id}/submissions/{slot_id}/{position_id}`. Two path params now.
    - Update the corresponding handler function to read both path params and call the service with `(slot_id, position_id)`.
  Verify: `go build ./...`.
- [ ] 4.2 In `backend/internal/handler/response.go`:
    - Rename the type `templateShiftResponse` → `qualifiedShiftResponse`.
    - Drop the JSON fields `id`, `template_id`, `created_at`, `updated_at` from the response shape per design D-2; keep only `slot_id`, `position_id`, `weekday`, `start_time`, `end_time`, `required_headcount`.
    - Update the constructor accordingly.
  Verify: same.
- [ ] 4.3 Update handler tests in `backend/internal/handler/publication_test.go` and `template_test.go`:
    - Replace `TemplateShiftID` body construction with `{ slot_id, position_id }`.
    - Replace path `/submissions/{n}` with `/submissions/{slot}/{position}`.
    - Add a regression-guard test: `POST /publications/{id}/assignments` with body `{"user_id": ..., "template_shift_id": 7}` (no `slot_id`/`position_id`) returns 400 `INVALID_REQUEST`.
  Verify: `go test ./internal/handler -count=1`.

## 5. Backend audit cleanup

- [ ] 5.1 Open `backend/internal/audit/audit.go`. If any constants, comments, or metadata field names mention `template_shift` or `TemplateShift`, rename to slot+position. Audit-action string values themselves SHALL NOT change (e.g., `assignment.create` stays). Verify: `go test ./internal/audit -count=1`.

## 6. Frontend type rename

- [ ] 6.1 In `frontend/src/lib/types.ts`: rename type `TemplateShift` → `QualifiedShift` with the new shape (no `id`, no `template_id`, no timestamps). Verify: `pnpm build` — expect compile failures cascading across consumers.
- [ ] 6.2 In `frontend/src/lib/queries.ts`:
    - Update query and mutation payloads: `template_shift_id` removed, replaced by `{ slot_id, position_id }`.
    - Update DELETE mutation URL to `/publications/{id}/submissions/{slot_id}/{position_id}` shape.
  Verify: `pnpm test` for queries.test.ts.

## 7. Frontend component renames

- [ ] 7.1 Rename `frontend/src/components/templates/template-shift-dialog.tsx` → `slot-position-dialog.tsx`; rename the exported component `TemplateShiftDialog` → `SlotPositionDialog`. Update internal `TemplateShiftFormValues` / `TemplateShiftFormProps` types accordingly. Verify: `pnpm build`.
- [ ] 7.2 Rename `frontend/src/components/templates/delete-template-shift-dialog.tsx` → `delete-slot-position-dialog.tsx`; rename component accordingly.
- [ ] 7.3 Rename `frontend/src/components/templates/group-template-shifts.ts` → `group-qualified-shifts.ts`; export `groupQualifiedShiftsByWeekday` (the helper still operates on the same data; only naming changes to match the new type).
- [ ] 7.4 Rename `frontend/src/components/templates/template-schemas.ts` types: `TemplateShiftFormValues` → `SlotPositionFormValues`, `templateShiftFormSchema` → `slotPositionFormSchema`.
- [ ] 7.5 Update `frontend/src/components/availability/availability-grid.tsx` and `availability-grid.test.tsx`: imports of `TemplateShift` → `QualifiedShift`; usage adapted (no surrogate `id`, key by `${slot_id}-${position_id}`).
- [ ] 7.6 Sweep all `*.test.ts(x)` files matching the renamed types/components and update imports + assertions. Verify: `pnpm test`.

## 8. Spec text edits — already in this change's specs delta

This is informational; no separate task. The specs delta in `specs/scheduling/spec.md` rewords four MODIFIED requirements (Qualification gates, Availability window, Employee availability endpoints, Admin assignment endpoints) and the spec sync at archive time pushes those into `openspec/specs/scheduling/spec.md`.

## 9. Final verification

- [ ] 9.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0.
- [ ] 9.2 Frontend clean: `cd frontend && pnpm lint && pnpm test && pnpm build` — every step exits 0.
- [ ] 9.3 Migrations roundtrip clean: `make migrate-down && make migrate-up && make migrate-down && make migrate-up` (no migration in this change, but exercise the full ladder to confirm nothing else broke).
- [ ] 9.4 Smoke test (manual):
    - (a) `make migrate-down && make migrate-up && make seed SCENARIO=full`. Login as `employee1@example.com` / `pa55word`. Open the availability page; confirm the grid loads (uses the renamed `QualifiedShift`).
    - (b) Tick a slot-position; confirm the network request POSTs `{ slot_id, position_id }` (no `template_shift_id`).
    - (c) Un-tick the same; confirm DELETE goes to `/publications/{id}/submissions/{slot_id}/{position_id}`.
    - (d) Login as admin; create an assignment via UI; confirm POST body uses `{ user_id, slot_id, position_id }`.
    - (e) curl `POST /api/publications/1/assignments` with `{"user_id": 1, "template_shift_id": 1}` (no slot/position fields). Confirm 400 `INVALID_REQUEST`.
- [ ] 9.5 Confirm no source file matches `grep -RIn "template_shift\|TemplateShift" backend frontend --include="*.go" --include="*.ts" --include="*.tsx"` outside of comments referring to historical context (if any). Ideally zero matches.
- [ ] 9.6 Confirm CI is green on the `change/drop-template-shift-compat` branch: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`.
