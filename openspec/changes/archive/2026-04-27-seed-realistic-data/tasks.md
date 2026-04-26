## 1. Code-gen pipeline

- [x] 1.1 Add `openspec/changes/seed-realistic-data/generate_realistic_seed.py` that reads `processed.local.csv` from project root, drops NetID/姓名/邮箱/角色 columns, parses each row's 5 weekday-list cells (Chinese full-width "：" separators), runs a deterministic `random.Random(20260427)` to assign archetypes per the D-2 distribution targets, verifies the per-weekday senior-coverage constraint (re-rolling the seed if violated), and emits `backend/cmd/seed/scenarios/realistic.go` with the literal slice contents. Verify by running `python3 openspec/changes/seed-realistic-data/generate_realistic_seed.py` and inspecting the produced Go file diff.
- [x] 1.2 Run the script once and commit the resulting `backend/cmd/seed/scenarios/realistic.go`. Verify by `cd backend && go build ./...`.

## 2. Realistic scenario implementation

- [x] 2.1 In `backend/cmd/seed/scenarios/realistic.go`, declare the `realisticEmployee` struct, the `archetype` enum (FrontSenior / FrontJunior / FieldSenior / FieldJunior), the `realisticTimeSlots [5]struct{Start,End string}` table, the `realisticEmployees [42]realisticEmployee` literal (output of step 1.1), and the `realisticAssignmentSeed` constant.
- [x] 2.2 Implement `RunRealistic(ctx, tx, opts)` per design D-5: insert bootstrap admin, 4 positions, 42 users (bcrypt `pa55word`, `status='active'`), `user_positions` per archetype, 1 template "Realistic Rota" with `is_locked=true`, 35 `template_slots` (5 time blocks × 7 weekdays), 70 `template_slot_positions` (daytime `{前台负责人 × 1, 前台助理 × 2}`, evening `{外勤负责人 × 1, 外勤助理 × 1}`), 1 publication "Realistic Rota Week" resolving to effective `ASSIGNING`, and per-row availability submissions filtered by domain (D-6).
- [x] 2.3 Wire `realistic` into `backend/cmd/seed/scenarios/common.go`'s dispatcher alongside `basic`/`full`/`stress`. Verify by `cd backend && go vet ./...`.
- [x] 2.4 Add `"realistic"` to the scenario `IsValid` allowlist in `backend/cmd/seed/main.go`. Verify by `cd backend && go build ./...`.

## 3. Spec sync

- [x] 3.1 Confirm the change-folder spec delta at `openspec/changes/seed-realistic-data/specs/dev-tooling/spec.md` matches the implemented behavior (4 scenarios, per-slot wording fix, new `realistic` scenario block). Do not edit `openspec/specs/dev-tooling/spec.md` directly — `/opsx:archive` will sync it.

## 4. Tests and smoke

- [x] 4.1 Existing `backend/cmd/seed/main_test.go` continues to pass (production-guard test). Verify by `cd backend && go test ./cmd/seed/...`.
- [x] 4.2 Manual smoke: with local Postgres up and `processed.local.csv` present, run `make seed SCENARIO=realistic`, then verify row counts: `SELECT COUNT(*) FROM users` = 43; `FROM positions` = 4; `FROM template_slots` = 35; `FROM template_slot_positions` = 70; `FROM publications` = 1 (effective `ASSIGNING`); `FROM assignments` = 0; `FROM availability_submissions` ≈ several hundred. Log the post-domain-filter drop count for visibility.
- [x] 4.3 Run full backend gate from the project root: `cd backend && go build ./... && go vet ./... && go test ./...`. All clean.
