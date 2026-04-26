## Why

The existing `full` and `stress` seed scenarios are synthetic — `full` has 8 employees with toy availability, `stress` has 50 employees whose availability is generated from a deterministic `(employee + slot) % 5 < 3` formula. Synthetic randomness has predictable distribution and doesn't reproduce the kinds of edge cases real users create: clusters of "everybody available Mon morning, nobody on Sun evening", senior staff who are sparse but qualified across roles, weekend-only volunteers, and so on. Manual smoke testing the auto-assigner / leave / shift-change flows on synthetic data hides these patterns.

A small de-identified dataset from a real cohort already exists on disk at `processed.local.csv` (gitignored, ~42 rows). This change introduces a new `realistic` seed scenario that bakes the *availability patterns* from that file (weekday vectors per time-slot column) into the seed binary, with all identifying columns stripped at code-gen time. The committed Go code holds: row index → fixed anonymous identity (`employee01..42` / Chinese display name / `employee01@example.com`) + a fixed-RNG-derived role archetype + the original weekday lists. No NetID, no real name, no real email is ever committed.

The slot-composition rules and position roster are explicit in the user's brief: 4 positions (前台负责人, 前台助理, 外勤负责人, 外勤助理); daytime slots staff `{前台负责人 × 1, 前台助理 × 2}`; the 19:00-21:00 evening slot staffs `{外勤负责人 × 1, 外勤助理 × 1}`; the org runs all 7 weekdays; ~75 % of staff are 前台-domain, ~25 % 外勤; within each domain a senior is also qualified for the assistant role of the same domain.

## What Changes

- **New seed scenario:** `make seed SCENARIO=realistic`. End state mirrors `full`'s shape (one publication in effective `ASSIGNING`, no assignments yet) but with realistic data:
  - **42 employees** with anonymized identities `employee01..42` / 中文显示名 `员工 1..42` / `employee01@example.com..employee42@example.com`. All `status='active'` with bcrypt-hashed `pa55word`.
  - **4 positions:** 前台负责人, 前台助理, 外勤负责人, 外勤助理.
  - **`user_positions` per archetype**, fixed by a deterministic RNG seed at code-gen time so re-running the seed always yields the same assignment:
    - 前台-senior → `{前台负责人, 前台助理}`
    - 前台-junior → `{前台助理}`
    - 外勤-senior → `{外勤负责人, 外勤助理}`
    - 外勤-junior → `{外勤助理}`
    Distribution: ~75 % 前台 / ~25 % 外勤; within each domain a balanced senior/junior split that keeps lead coverage feasible — target: ≥ 1 senior per domain available on every `(weekday, domain)` cell where the source data has *any* availability. Cells with zero source-data availability (e.g., Sunday daytime in the front domain — `processed.local.csv` happens to have 0 rows with weekday 7 in any of the four daytime columns) stay uncovered. This is intentional; real-cohort gaps are part of the smoke-testing surface and let auto-assign exercise its empty-slot path.
  - **1 template** "Realistic Rota" with **35 slots** = 5 time blocks × 7 weekdays:
    - 09:00-10:00, 10:00-12:00, 13:30-16:10, 16:10-18:00 → composition `{前台负责人 × 1, 前台助理 × 2}` per weekday × 4 = 28 daytime slots
    - 19:00-21:00 → composition `{外勤负责人 × 1, 外勤助理 × 1}` per weekday × 1 = 7 evening slots
  - **1 publication** "Realistic Rota Week" referencing the template, with `submission_start_at` in the past, `submission_end_at` in the past, `planned_active_from` and `planned_active_until` 7 days apart in the future. Effective state resolves to `ASSIGNING`.
  - **Per-row availability submissions** per the new per-slot model: for each CSV row × each of the 5 time-slot columns × each weekday number in the comma-separated cell, insert one `availability_submissions(publication_id, user_id, slot_id)` row. The user is filtered to slots whose composition has at least one position in their `user_positions` (i.e., 前台 employees never submit for the evening 外勤 slot, and vice versa). The CSV row's "available weekdays" list is the source of truth for what gets ticked.
  - **Zero assignments**, so the developer can immediately call `POST /publications/{id}/auto-assign` to exercise the new MCMF.
- **Seed binary updates:**
  - New `backend/cmd/seed/scenarios/realistic.go` with the scenario `Run` function and the embedded data (anonymized identity tuples + archetype + weekday lists) as a `[]realisticEmployee` slice literal.
  - The slice literal is the result of a one-shot Python code-gen step that reads `processed.local.csv`, anonymizes, runs a fixed-seed RNG to assign archetypes per the distribution rules, and emits the Go literal. The Python script lives in the change folder for reference but is *not* part of the apply (its output — the `.go` file — is what ships).
  - `backend/cmd/seed/scenarios/common.go` dispatcher gains the `realistic` case alongside the existing three.
  - `backend/cmd/seed/main.go` accepts `--scenario=realistic` (current `IsValid` allowlist expands).
- **Spec changes** (`dev-tooling`):
  - The single requirement *Local-development data seeding command* reworded: "three named scenarios" → "four named scenarios"; new scenario added to the requirement list with its description.
  - The scenario *make seed SCENARIO=full provides ASSIGNING-state data* has stale wording from before `simplify-availability-submission` ("60% of qualified `(slot, position)` pairs have an `availability_submissions` row per employee"). Reword to per-slot semantics ("roughly 60% of qualified slots have an `availability_submissions` row per employee") so the spec doesn't contradict the now-current data model. This is a small fix tacked on; the `full` scenario's actual seeded behavior already matches the new model post-simplify.
  - Add a new scenario block describing what `realistic` produces.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `dev-tooling`: adds `realistic` to the seed-scenario list and refreshes the `full`-scenario wording for the per-slot submission model.

## Non-goals

- **Committing the source CSV.** `processed.local.csv` stays gitignored. Only the anonymized derivative is committed (as Go literals).
- **Committing real names / NetIDs / emails.** All identifying columns are stripped at code-gen time. Anyone reviewing the diff sees only `employeeNN`-style identifiers.
- **Building a CSV-import CLI.** This is a *seed scenario* — bake-once at code-gen, ship the Go code. The earlier "import-availability tool" idea is out of scope.
- **Replacing the existing `stress` scenario.** `stress` keeps its synthetic-50-employee shape for performance / multi-publication testing. `realistic` is additive.
- **Live re-randomization at seed runtime.** Archetype assignment is fixed at code-gen so seed runs are reproducible. Re-running with a different distribution requires regenerating the slice literal (a one-shot operation).
- **Updating Excel/CSV ingestion paths.** The realistic seed is the only consumer of the source data.
- **Validating the source CSV at runtime.** The code-gen step does the validation once; the resulting Go literal is statically typed.

## Impact

- **Backend code:**
  - New file `backend/cmd/seed/scenarios/realistic.go` (~250 lines incl. embedded data: 42 entries × ~5 fields).
  - Modified `backend/cmd/seed/scenarios/common.go` — dispatcher gains `case "realistic"`.
  - Modified `backend/cmd/seed/main.go` — `IsValid`'s allowlist gains `"realistic"`.
  - Modified `backend/cmd/seed/main_test.go` — happy-path test for the production guard already exists; add coverage for the new case in dispatcher tests if any.
- **Spec:**
  - Modified `dev-tooling`'s single requirement: list "four scenarios", add new scenario block, refresh `full` scenario wording.
- **Source-data file:** `processed.local.csv` continues to be gitignored. The change folder includes (under `openspec/changes/seed-realistic-data/`) a `generate_realistic_seed.py` reference script that produces the Go literal — this is documentation; the build does not depend on it.
- **Tests:** existing seed unit tests pass; the integration smoke test runs `make seed SCENARIO=realistic` end-to-end and asserts row counts.
- **No new third-party dependencies.**
- **No frontend changes.**
- **No infra / config changes.**

## Risks / safeguards

- **Risk:** an employee's archetype is misclassified at code-gen time (e.g., someone marked 外勤-junior who actually does 前台 in real life). **Mitigation:** the fixed seed makes the assignment reproducible, and the diff is human-readable — anyone with org context can spot a mis-classification and re-run code-gen with a different seed or hand-tweak the slice literal.
- **Risk:** schema drift between this seed and the live model. **Mitigation:** the seed is exercised by `make seed` in dev daily; drift surfaces as compile or runtime failures fast.
- **Risk:** PII leaks via the Go literal. **Mitigation:** the code-gen explicitly drops the source CSV's NetID / 姓名 / 邮箱 columns before producing output; the only emitted identifiers are deterministic synthetic strings (`employeeNN`).
- **Risk:** the `full` scenario's spec wording fix (per-slot vs per-`(slot, position)`) drags in additional scope. **Mitigation:** the change to the spec text is a one-paragraph edit; the actual seeded behavior already matches per-slot post `simplify-availability-submission`.
