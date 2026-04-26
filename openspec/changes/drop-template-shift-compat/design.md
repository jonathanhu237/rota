## Context

Three changes ago (`refactor-to-slot-position-model`), the database moved from a single `template_shifts` table to two tables: `template_slots` (the time grid) and `template_slot_positions` (which positions sit on which slot). The Go and TypeScript codebases were partially migrated — call sites that sat directly on top of the new schema were updated, but the layer "above" them still spoke the old language: a single `template_shift_id` (which under the hood is `template_slot_positions.id`) was kept as an alias on request bodies and response payloads, and a `model.TemplateShift` Go struct continued to represent the joined `(slot, position)` shape.

This change finishes the migration. The legacy alias and its supporting machinery are removed; what stays in user-facing copy is the word "shift" itself, because that is still the natural English term for "a row a user can sign up for". Internal Go and TypeScript code uses `slot` and `position` directly.

## Goals / Non-Goals

**Goals:**

- Single canonical identifier for a slot-position row throughout the codebase: `(slot_id, position_id)`.
- Zero references to `template_shift_id`, `model.TemplateShift`, or `ErrTemplateShiftNotFound` after the change lands.
- Keep public API path `/publications/{id}/shifts/me` unchanged — the URL is user-facing.
- Keep all behavior identical: same validations, same error codes (with one exception, see D-3), same audit events.
- Compile-time guarantee that nothing partial ships.

**Non-Goals:**

- Renaming the user-facing word "shift".
- Schema changes.
- New endpoints or new behavior.
- Dual-supporting both shapes during transition (cut clean — no production data, frontend is updated in the same change).

## Decisions

### D-1. Replace `model.TemplateShift` with `model.QualifiedShift`

**Decision:** introduce a new struct `model.QualifiedShift` to hold the joined-shape representation that powers `/shifts/me` responses. Fields: `SlotID`, `PositionID`, `Weekday`, `StartTime`, `EndTime`, `RequiredHeadcount`. The struct intentionally omits a single surrogate `ID` field because no caller needs to identify a row by anything other than `(slot_id, position_id)` after the cleanup.

**Alternatives considered:**

- *Drop the struct entirely; build the shape inline in handlers from `model.TemplateSlot` + `model.TemplateSlotPosition`*: spreads join logic across handlers; harder to keep response shape consistent. Rejected.
- *Rename in place to `model.SlotPosition` or `model.SlotPositionRow`*: those names already exist or strongly evoke `template_slot_positions` rows (which carry only `id, slot_id, position_id, required_headcount`, not weekday/time). The "qualified shift" name signals "a thing a user can sign up for", which is the actual semantic. Chosen.

### D-2. Drop the surrogate `id` from `/shifts/me` responses

**Decision:** the response shape changes from

```json
{ "id": 7, "template_id": 1, "weekday": 1, "start_time": "09:00",
  "end_time": "12:00", "position_id": 5, "required_headcount": 1, ... }
```

to

```json
{ "slot_id": 3, "position_id": 5, "weekday": 1, "start_time": "09:00",
  "end_time": "12:00", "required_headcount": 1 }
```

`template_id` is also dropped (caller already knows the publication, hence template; redundant). `created_at` / `updated_at` are dropped (employee-facing list does not display them).

The frontend's existing call sites use `id` today as a stable React key. After the change they switch to a composite key `${slot_id}-${position_id}` or use `slot_id` directly when listing within a single position context. No information is lost; only the surrogate identifier is.

**Alternative:** keep `id` as the `template_slot_positions.id`. Rejected because the goal is to eliminate that surrogate identifier from the surface area entirely; otherwise the cleanup is partial.

### D-3. `template_shift_id` rejection becomes `INVALID_REQUEST` (was implicit)

**Decision:** before this change, sending `{"template_shift_id": 7}` to `POST /publications/{id}/submissions` succeeded by silently resolving to `(slot_id, position_id)`. After the change, the field is unknown to the JSON decoder and is silently ignored; if no other recognised field is present, the request is rejected as `INVALID_REQUEST` (400) with no body change required. This is the existing rejection path for unrecognized requests — no new error code.

### D-4. Service-layer signature renames

**Decision:** rename methods that contain "shift" in their name when they operate on the slot-position model:

- `publicationService.ListQualifiedPublicationShifts` → `ListQualifiedPublicationSlotPositions`
- `repository.ListQualifiedPublicationShifts` → `ListQualifiedPublicationSlotPositions`
- `repository.GetTemplateShiftByID` → drop entirely (no callers after the resolver simplification)

The handler that powers `/publications/{id}/shifts/me` stays under that route; it calls the renamed service method internally and serializes via the new `qualifiedShiftResponse`.

### D-5. Frontend renames

**Decision:**

- `TemplateShift` type in `frontend/src/lib/types.ts` → `QualifiedShift` with the new shape (no `id`, no `template_id`, no timestamps; uses `slot_id + position_id`).
- `TemplateShiftDialog` component (admin template editor) is currently the dialog for editing one row of the template's slot/position grid. Rename to `SlotPositionDialog`.
- `delete-template-shift-dialog.tsx` → `delete-slot-position-dialog.tsx`.
- `groupTemplateShiftsByWeekday` helper → `groupQualifiedShiftsByWeekday` (keeps the user-facing "shifts" wording).
- `TemplateShiftFormValues` schema type → `SlotPositionFormValues`.
- `availability-grid.tsx` import sites and types updated to use `QualifiedShift`.

### D-6. Test strategy

**Decision:** rely on the compiler. Removing `model.TemplateShift` and `model.ErrTemplateShiftNotFound` and the field name `TemplateShiftID` from request types causes every leftover call site to fail compilation. The change is large in line count but mechanical; no test should require redesign — only renames.

For the one behavioral change point (D-3), add a single handler test ensuring `POST /publications/{id}/assignments` with only `template_shift_id` returns 400 `INVALID_REQUEST`. This is the regression-guard against accidentally re-introducing the legacy alias.

### D-7. Sequencing

The change touches ~20 backend files and ~13 frontend files. Tasks ordering:

1. Backend model deletions / renames (compile failures cascade).
2. Backend repository updates.
3. Backend service renames.
4. Backend handler updates.
5. Backend tests sweep.
6. Frontend type rename.
7. Frontend component renames + call site updates.
8. Frontend tests sweep.
9. Spec text edits.
10. Final verification.

This ordering keeps the "compile failure surface" pointing at the next file to fix until the change is complete.

## Risks / Trade-offs

- **Risk: rename churn obscures the small behavioral change in D-3.** → Mitigation: keep the regression-guard test in a dedicated commit chunk separate from the rename mechanics; reviewer can spot-check it specifically.
- **Risk: `id` removed from `/shifts/me` breaks an unnoticed React `key={shift.id}` site.** → Mitigation: the type rename forces compile-level visibility of every consumer; runtime behaviour is then verified by `pnpm test`.
- **Trade-off: deleting `model.TemplateShift` versus renaming it.** → Renaming would be safer but spreads "what does TemplateShift mean now" confusion. The clean delete + new struct under a new name forces us to think about each call site.
- **Trade-off: keep `/shifts/me` URL.** → Renaming would be more consistent, but the URL is user-facing (frontend route, possibly browser-bookmark-able) and "shift" remains a meaningful UX word. Net cost of the inconsistency is one English word in one URL; net cost of the rename would include extra spec churn and frontend route updates. Keep.

## Migration Plan

Single shipping unit. No DB migration. The frontend and backend changes land together on `change/drop-template-shift-compat` and merge to `main` atomically. Rollback is a `git revert` of the single feature-branch range; everything in this change is reversible by code revert (no data state changes).

## Open Questions

None.
