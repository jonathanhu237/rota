## Why

The `refactor-to-slot-position-model` change dropped the `template_shifts` database table and split it into `template_slots` + `template_slot_positions`, but the surrounding Go and TypeScript code still carries the old "shift" naming and an active compatibility layer that translates between two ways of identifying the same row in `template_slot_positions`:

- The new internal model: `(slot_id, position_id)` natural key.
- The legacy alias: `template_shift_id` (= `template_slot_positions.id` surrogate id), still accepted by request bodies and resolved through `repository.resolveAssignmentRef` + `repository.getTemplateSlotPositionPairByEntryID`.

Today this means readers see two parallel APIs for the same operation, plus ~50 lines of resolver code whose only job is to translate one into the other. There is no external API consumer (the frontend is the only client and we control both sides), so the legacy form has no audience. The spec already declared that `template_shift_id` SHALL NOT be accepted (see `specs/scheduling/spec.md` requirement *Admin assignment endpoints*), but the handler code does accept it — this change closes that drift.

The cleanup is pure refactoring: no user-visible behavior change, no schema migration. The win is a smaller surface area, one official identifier for a slot-position pair, and consistency between spec and code.

## What Changes

- **BREAKING (internal API request bodies):** the `template_shift_id` field on `POST /publications/{id}/submissions` and `POST /publications/{id}/assignments` is removed. Callers MUST send `{ slot_id, position_id }`. The frontend is updated in lockstep; no external clients exist.
- **Backend code removals:**
  - `backend/internal/repository/template_shift_compat.go` is deleted (it carries only one re-exported sentinel).
  - `model.TemplateShift` struct is removed; the join shape needed for `/shifts/me` and similar responses moves into a new `model.QualifiedShift` that exposes `slot_id`, `position_id`, weekday, start/end times, and required headcount — without the legacy single-id surrogate field.
  - `model.ErrTemplateShiftNotFound` is removed; callers use `model.ErrTemplateSlotPositionNotFound` (already present).
  - `repository.resolveAssignmentRef` loses its `templateShiftID` branch and becomes a slot-position-only resolver.
  - `repository.getTemplateSlotPositionPairByEntryID` is deleted (no callers after the resolver simplification).
  - `service.ErrTemplateShiftNotFound` alias is removed.
  - `service.publicationService.ListQualifiedPublicationShifts` (and any related signatures) are renamed to use `SlotPosition` language consistently.
- **Backend response shape:** the JSON returned by `GET /publications/{id}/shifts/me` no longer includes the legacy single `id` (which mapped to `template_slot_positions.id`); instead it surfaces `slot_id` + `position_id` as the identifying pair. The Go response type is renamed from `templateShiftResponse` to `qualifiedShiftResponse`.
- **Backend tests:** every test that constructed `TemplateShiftID` or referenced `model.TemplateShift` is updated to the new shape. Tests are exhaustive enough that compilation failure points to all required edits.
- **Frontend code rename:**
  - `TemplateShift` type in `lib/types.ts` → `QualifiedShift`.
  - `TemplateShiftDialog` component → `SlotPositionDialog` (or equivalent slot-positioned name; see design).
  - `delete-template-shift-dialog.tsx` → renamed.
  - `groupTemplateShiftsByWeekday` helper → renamed.
  - `TemplateShiftFormValues` schema type → renamed.
  - All call sites and tests follow.
- **Spec text:** requirements that mention "template_shift" in scenario WHEN/THEN clauses (e.g., *Qualification gates employee actions*, *Employee availability endpoints*) are reworded to refer to `(slot, position)` pairs instead.
- **API path `/publications/{id}/shifts/me` is kept.** "Shift" remains the user-facing word for "a row a user can sign up for"; only the wire shape and internal types change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: removes the legacy `template_shift_id` from request bodies, drops the `model.TemplateShift` Go type and all its companions, and rewrites a handful of requirement scenarios to use slot+position language. The contract change is internal; the user-facing roster/assignment behavior is unchanged.

## Non-goals

- **Renaming the user-facing word "shift".** The frontend continues to call user-signupable rows "shifts" in copy and route paths. Only the implementation language changes.
- **Database changes.** No migration. The DB is already on the slot+position model since `refactor-to-slot-position-model`.
- **Behavior changes.** No new endpoints, no changed validation, no different audit events. Removing the legacy field path means a malformed request that used `template_shift_id` will now be rejected as `INVALID_REQUEST`, but any current frontend send-site is updated to use the new shape in the same change.
- **Deprecation period.** No external clients exist; cut clean over rather than dual-supporting.
- **Audit-action renames.** Existing audit actions keep their string values (e.g., `assignment.create`); only Go-side metadata names align with the new model.

## Impact

- **Backend code:**
  - `backend/internal/model/template_shift.go` → deleted (or repurposed as `qualified_shift.go` if the join-shape struct stays).
  - `backend/internal/repository/template_shift_compat.go` → deleted.
  - `backend/internal/repository/assignment.go` → resolver simplified, helper deleted.
  - `backend/internal/repository/publication.go` → fields and method names renamed.
  - `backend/internal/service/template.go`, `service/publication.go` → renames + signature changes.
  - `backend/internal/handler/publication.go` → request body field removed, request shape narrowed.
  - `backend/internal/handler/response.go` → response Go type renamed; JSON omits legacy `id` field.
  - `backend/internal/audit/audit.go` → any leftover comment/constant referring to `template_shift_id` cleaned up.
  - All matching `_test.go` files updated.
- **Frontend code:**
  - Type/component renames listed above.
  - `lib/queries.ts` request payload shapes updated.
  - Tests follow.
- **Test impact:** mechanical but broad. Compile-time errors will surface every site that needs updating.
- **No third-party dependencies change.**
- **No documentation outside spec changes.** README is unaffected.

## Risks / safeguards

- **Risk:** mechanical rename misses one site, leaves dangling legacy references that compile but produce stale JSON. **Mitigation:** the deletion of `model.TemplateShift` and `model.ErrTemplateShiftNotFound` causes compile-time failures at every leftover call site — the change cannot land partial.
- **Risk:** rename churn obscures meaningful diffs in code review. **Mitigation:** the change description names every renamed type/file, and the tasks list ordering does deletions first, then renames, then internal cleanup.
- **Risk:** spec text edits introduce subtle scenario meaning changes. **Mitigation:** reword only the language; do not change the WHEN/THEN logic of any scenario.
