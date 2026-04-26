## Why

Today an `availability_submissions` row is keyed by `(publication_id, user_id, slot_id, position_id)`. The UI follows: for each `(slot, position)` pair the viewer is qualified for, the availability grid renders an independent checkbox. An employee qualified for both 前台负责人 and 前台助理 sees two separate boxes for the same Mon 09:00–10:00 slot — they have to tick both to express "I'm free at this time". Worse, ticking only one is interpreted as "I want to do role X but not role Y" — a preference signal nobody actually wants to send. From the employee's mental model, the question is just "are you free?", not "for which role?". The system already knows which roles each user can fill via `user_positions`; making the user re-state that in their availability submission is redundant and confusing.

This change collapses availability to per-slot. An employee ticks one box per `(slot)` to mean "I am free in this time block". Auto-assign continues to use `user_positions` to decide which `(slot, position)` slot-position cell each candidate fills. No data is lost — qualification information already lives in `user_positions`; the submission no longer duplicates it.

There are no production submissions today, so this is a clean schema migration with no data to massage.

## What Changes

- **BREAKING (DB schema):** `availability_submissions.position_id` column is removed. The natural-key uniqueness shrinks from `(publication_id, user_id, slot_id, position_id)` to `(publication_id, user_id, slot_id)`. The composite FK to `template_slot_positions(slot_id, position_id)` is removed; the remaining FKs (publication, user, slot) stay. Indexes referencing position drop or simplify accordingly.
- **API request / response shapes**:
  - `POST /publications/{id}/submissions` body shrinks from `{slot_id, position_id}` to `{slot_id}`.
  - `DELETE /publications/{id}/submissions/{slot_id}/{position_id}` URL becomes `DELETE /publications/{id}/submissions/{slot_id}`.
  - `GET /publications/{id}/shifts/me` returns one row per `slot` the viewer is qualified for (i.e., the slot has at least one position in the viewer's `user_positions`), not one row per `(slot, position)` pair. The response shape carries `slot_id, weekday, start_time, end_time` and a `position_summary` (the slot's composition for display), but no per-position checkbox.
  - `GET /publications/{id}/submissions/me` returns `slot_id`s only.
- **Auto-assign MCMF candidate pool** is rederived: a candidate is `(user, slot)` with at least one matching position in `slot.composition ∩ user_positions`. The graph builds `(user, slot)` → `(slot, position)` edges only for positions the user is qualified for. The "submission whose `(user_id, position_id)` is no longer in `user_positions`" filter becomes "user no longer has any qualifying position for the slot's composition".
- **Frontend availability grid**: each weekday section renders one checkbox per slot the viewer can fill, with a small caption listing the slot's composition (e.g., "前台负责人 × 1 / 前台助理 × 2") for context. Ticking the box submits availability for the slot only.
- **Spec changes** in `scheduling`:
  - *Availability submission data model* — drops `position_id`, simplified uniqueness, simpler FK story.
  - *Availability window* — DELETE URL updates to single-id form.
  - *Employee availability endpoints* — body and response shape updates.
  - *Auto-assign replaces the full assignment set via MCMF* — candidate-pool wording updates to per-slot semantics; graph descriptions adjust.
  - *Qualification gates employee actions* — the scenario that says "submits availability for a `(slot, position)` pair whose `position_id` is not in their `user_positions`" rewrites as "submits availability for a `slot` whose composition has no overlap with their `user_positions`".
- **Migration:** a single goose migration drops the column + reshapes the unique constraint. No data preservation logic — there are no production rows.
- **Tests:** every test that built submissions with `(slot_id, position_id)` rewrites to `(slot_id)`; every test that asserted candidate-pool composition rewrites for the new derivation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: simplifies the `availability_submissions` data model and downstream endpoints / auto-assign / UI to a per-slot model. User-visible behavior preserved (submitting availability still gates auto-assign as input); the cosmetic-but-confusing "tick one box per role" UX is replaced by "tick one box per time slot".

## Non-goals

- **Adding a per-role preference signal.** Employees no longer say "I prefer 负责人 over 助理 in this slot". If we ever need that, it's a separate change introducing an explicit preference table — not stuffing it back into the submission.
- **Changing how assignments are stored.** `assignments` keeps `(publication_id, user_id, slot_id, position_id)` — the actual job assignment is still per role. Only the *availability* model loses position.
- **Changing `assignment_overrides`, shift-change, or leave models.** Those reference `assignment_id` (which still points at a `(slot, position)` cell). Untouched.
- **Live data migration.** Dev databases get reset; no production rows exist.
- **UI shape changes beyond the availability grid.** Admin assignment board, leave UI, shift-change UI all stay as-is — they operate on assignments, not submissions.

## Impact

- **Backend code:**
  - New migration: drops `position_id` column, replaces unique index, removes the composite FK to `template_slot_positions`. Down recreates them (see design D-3 for the round-trip story; with no production data the Down direction is best-effort).
  - `backend/internal/model/publication.go` (or wherever the submission struct lives) — drop `PositionID` field on `AvailabilitySubmission`.
  - `backend/internal/repository/publication.go` (and any submission-touching repo files): SELECT/INSERT/DELETE adjust to the new column set; unique-violation mapping continues to work.
  - `backend/internal/service/publication.go`: `CreateAvailabilitySubmissionInput` and `DeleteAvailabilitySubmissionInput` lose `PositionID`; auto-assign candidate-pool building rederives per design D-2.
  - `backend/internal/handler/publication.go`: submission body drops `position_id`; DELETE URL drops the trailing `position_id` segment; `/shifts/me` and `/submissions/me` response shapes simplify.
  - Tests across model / repository / service / handler update accordingly.
- **Frontend code:**
  - `frontend/src/components/availability/availability-grid.tsx` collapses the per-(slot, position) checkbox loop into a per-slot loop. Each box shows the slot's composition as a caption.
  - `frontend/src/components/templates/group-qualified-shifts.ts` returns one entry per slot the viewer can fill (with composition summary), not one per `(slot, position)`.
  - `frontend/src/lib/types.ts`: `QualifiedShift` (or whatever the per-row type is now) drops `position_id`; gains a `composition` field summarizing the slot's positions.
  - `frontend/src/lib/queries.ts`: submission mutation payload + DELETE URL update.
  - i18n: replace `availability.shift.summary` (currently per-position) with a per-slot composition summary string.
- **Spec:** five `scheduling` requirements modify (data model, window, endpoints, auto-assign, qualification gates). No new requirements.
- **No new third-party deps. No infra changes.**

## Risks / safeguards

- **Risk:** auto-assign MCMF graph re-derivation has subtle edge cases (a `(user, slot)` candidate with multiple qualifying positions becomes a one-of-many edge fan-out). **Mitigation:** the existing per-slot-uniqueness intermediate node already enforces "user takes at most one position per slot"; the new graph is a strict simplification.
- **Risk:** test rewrites are wide. **Mitigation:** the migration's column drop causes compile-time failures at every leftover `PositionID` reference, surfacing the work to do.
- **Risk:** some user-facing copy currently references roles in availability flow. **Mitigation:** the i18n update sweeps these; the per-slot caption preserves the role-display information for context (just decoupled from the checkbox itself).
- **Risk:** a future feature might want the per-role preference back. **Mitigation:** if so, add a separate `availability_preferences` table — don't conflate availability ("am I free?") with preference ("which role do I prefer?").
