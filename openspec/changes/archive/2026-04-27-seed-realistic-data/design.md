## Context

The seed binary already has three scenarios — `basic`, `full`, `stress` — that produce synthetic data for local development. They cover different sizes (5 / 8 / 50 employees) but all use formulaic availability patterns. Real cohorts have lumpier patterns: bursts of availability mid-week, sparse Sundays, senior staff with selective availability that doesn't match a `% modulus` rule. To smoke-test auto-assign / shift-change / leave flows the way they'll actually run in production, we want a seed scenario whose availability vectors come from a real dataset.

The dataset exists at `processed.local.csv` in the project root. It is gitignored — already de-identified upstream of any process we run, but we treat it as PII and never commit any column from it that could re-identify a person. This change runs a one-shot anonymization pass over the CSV at code-gen time and bakes the *availability vectors* into Go source. The resulting `realistic.go` carries only synthetic identifiers and the original weekday lists.

## Goals / Non-Goals

**Goals:**

- A `realistic` seed scenario that produces 42 employees + ASSIGNING-state publication ready for auto-assign smoke testing.
- Source CSV stays gitignored; committed Go file contains only synthetic identities + availability vectors + deterministic-RNG-derived archetypes.
- Slot composition follows the user's spec exactly: daytime = `{前台负责人 × 1, 前台助理 × 2}`; 19:00-21:00 = `{外勤负责人 × 1, 外勤助理 × 1}`; all 7 weekdays.
- Archetype distribution: ~75 % 前台 / ~25 % 外勤; senior/junior split keeps slot lead-coverage feasible *where the source data has any availability for that weekday/domain cell*. Cells with zero source-data coverage are left empty — real-cohort gaps are part of the smoke-testing surface.
- Reproducible shape: re-running the seed yields the same roster, archetypes, slot definitions, and availability vectors. Runtime timestamps and bcrypt salts may differ, matching the existing seed helper behavior.

**Non-Goals:**

- Committing the CSV.
- Building runtime CSV-loading.
- Replacing `full` or `stress`.
- Live re-randomization.

## Decisions

### D-1. Anonymization scheme

Each CSV row's index `i` (1..42) maps deterministically to a synthetic identity:

| field | value |
|---|---|
| internal slug | `employee01`, `employee02`, ... `employee42` |
| display name (Chinese) | `员工 1`, `员工 2`, ... `员工 42` |
| email | `employee01@example.com`, ... `employee42@example.com` |

Original NetID / 姓名 / 邮箱 columns are dropped at code-gen — they never enter the committed Go file or any artifact in the change folder.

The original 角色 column (`普通助理` / `资深助理`) is also dropped: per the user's instruction, the new role assignment is independent of the legacy column, drawn from a deterministic RNG instead.

### D-2. Archetype distribution

Four archetypes correspond to four `user_positions` sets:

| archetype | user_positions |
|---|---|
| FRONT_SENIOR (`前台-senior`) | `{前台负责人, 前台助理}` |
| FRONT_JUNIOR (`前台-junior`) | `{前台助理}` |
| FIELD_SENIOR (`外勤-senior`) | `{外勤负责人, 外勤助理}` |
| FIELD_JUNIOR (`外勤-junior`) | `{外勤助理}` |

For 42 employees the target distribution:

```
前台 total ≈ 32 (76 %)
   FRONT_SENIOR  ≈ 10
   FRONT_JUNIOR  ≈ 22
外勤 total ≈ 10 (24 %)
   FIELD_SENIOR  ≈ 4
   FIELD_JUNIOR  ≈ 6
```

Two constraints to verify post-assignment:

1. **Senior coverage where source data permits.** For every `(weekday, domain)` cell where the source CSV has at least one row with availability, at least one senior of that domain SHALL be available. The code-gen runs this feasibility check after sampling; if any reachable cell fails, the RNG seed bumps and we resample. Cells with zero source-data availability (e.g., Sunday daytime in the front domain — empirically `processed.local.csv` has 0 rows with weekday 7 in any of the four daytime columns) are not reachable and are left uncovered. This is intentional: real cohorts have gaps, and smoke-testing auto-assign with realistic gaps exercises the empty-slot path that synthetic scenarios paper over.
2. **No empty domain.** At least 4 FRONT_SENIOR and 2 FIELD_SENIOR exist.

The RNG seed is a constant in `realistic.go` (e.g., `realisticAssignmentSeed = 20260427`) so the assignment is reproducible across machines.

### D-3. Code-gen pipeline (one-shot)

```
processed.local.csv  ──▶  generate_realistic_seed.py
                                 │
                                 │  reads CSV
                                 │  drops NetID/姓名/邮箱 columns
                                 │  parses weekday-list cells
                                 │  runs deterministic RNG for archetype
                                 │  emits Go slice literal
                                 ▼
                         realistic.go  ──▶  go build / go test
```

The Python script (`openspec/changes/seed-realistic-data/generate_realistic_seed.py`) reads `processed.local.csv` from the project root, applies the anonymization + archetype rules above, and writes `backend/cmd/seed/scenarios/realistic.go`.

The script is a one-shot operation: run it once during apply, commit the generated `.go` file, and ship. The script itself stays in the change folder (gets archived with the rest of the change) for posterity / reproducibility — anyone wanting to regenerate with a different seed can re-run it. **The build does not depend on the script**; CI builds from the `.go` file alone.

### D-4. realistic.go shape

```go
package scenarios

// realisticEmployee is the embedded shape produced by code-gen.
// See openspec/changes/archive/<date>-seed-realistic-data/generate_realistic_seed.py.
type realisticEmployee struct {
    Slug     string  // "employee01"
    Display  string  // "员工 1"
    Email    string  // "employee01@example.com"
    Arch     archetype
    Weekdays [5][]int // one slice per time-slot column, weekday numbers (1-7)
}

type archetype int

const (
    FrontSenior archetype = iota
    FrontJunior
    FieldSenior
    FieldJunior
)

var realisticTimeSlots = [5]struct {
    Start, End string
}{
    {"09:00", "10:00"},
    {"10:00", "12:00"},
    {"13:30", "16:10"},
    {"16:10", "18:00"},
    {"19:00", "21:00"},
}

var realisticEmployees = [42]realisticEmployee{
    {"employee01", "员工 1", "employee01@example.com", FrontSenior, [5][]int{
        {1, 2, 4, 5, 6}, {1, 2, 4, 5, 6}, {1, 2, 4}, {1, 2, 3, 4, 5}, {1, 2, 3, 4, 5, 6, 7},
    }},
    // ... 41 more, code-gen output
}

// RunRealistic seeds the realistic scenario.
func RunRealistic(ctx context.Context, tx *sql.Tx, opts Options) error {
    // ... per design D-5
}
```

### D-5. Seeding sequence

`RunRealistic` performs (all inside the existing wipe-then-seed transaction):

1. **Insert bootstrap admin** (reused via `insertUsers(opts, 0)` or equivalent — admin only, no synthetic employees from `insertUsers`).
2. **Insert 4 positions** in this order: 前台负责人, 前台助理, 外勤负责人, 外勤助理. Capture the IDs.
3. **Insert 42 users** by iterating `realisticEmployees`. Each row's email/name/password follow the synthetic identifiers. Capture user IDs in an array indexed by row.
4. **Insert `user_positions`** by iterating `realisticEmployees` again — for each archetype, look up the matching position IDs and insert one row per position-in-archetype.
5. **Insert 1 template** "Realistic Rota", `is_locked = true`.
6. **Insert 35 template_slots** = 5 time blocks × 7 weekdays. Capture each slot's ID in a `slotID[time_index][weekday]` 2-d lookup.
7. **Insert 70 template_slot_positions:**
   - For weekday 1..7, time index 0..3 (daytime): insert `{slot, 前台负责人, 1}` and `{slot, 前台助理, 2}`.
   - For weekday 1..7, time index 4 (evening): insert `{slot, 外勤负责人, 1}` and `{slot, 外勤助理, 1}`.
8. **Insert 1 publication** "Realistic Rota Week" referencing the template with timestamps that resolve effective state to `ASSIGNING`. Apply the existing on-create D2 sweep helper if needed (it is, since prior scenarios may leave a non-ENDED row).
9. **Insert availability submissions** by iterating `realisticEmployees` × 5 time-slot columns × weekday list. For each `(employee, time_index, weekday)`:
   - Look up `slotID[time_index][weekday]`.
   - Determine domain: time_index 0..3 ⇒ 前台 domain; time_index 4 ⇒ 外勤 domain.
   - If the employee's archetype is in the wrong domain (e.g., FrontSenior submitting for an evening slot), skip — `simplify-availability-submission` rejects such submissions at the service layer, and we don't want to seed invalid rows.
   - Insert one `availability_submissions(publication_id, user_id, slot_id)` row.
10. **No assignments** are created. The developer runs auto-assign manually after seeding.

### D-6. Domain-cross filter

The CSV does not encode "which domain". Some rows have evening-time-slot weekdays even for what archetype-RNG ends up classifying as 前台. Per D-5 step 9, we **drop** any submission where the time slot's domain doesn't match the employee's archetype domain. This is the only place where the seed deviates from "verbatim from CSV" — and it has to deviate, because the simplified service-layer qualification check would reject the row.

The expected drop rate is small (front-office staff occasionally sign up for evening slots in the source data, but they aren't qualified to fill the evening composition under the new role model). The smoke test in §7.4 logs the drop count as informational.

### D-7. Spec text adjustments

Single requirement *Local-development data seeding command* in `dev-tooling`:

- Change "three named scenarios" → "four named scenarios".
- The existing scenario block *make seed SCENARIO=full provides ASSIGNING-state data* contains stale wording from before `simplify-availability-submission`: "roughly 60% of qualified `(slot, position)` pairs have an `availability_submissions` row per employee". Reword to "roughly 60% of qualified slots have an `availability_submissions` row per employee" — the data-model migration already changed the actual seeded behavior to per-slot.
- Add a new scenario block *make seed SCENARIO=realistic provides anonymized real-cohort data*: 42 employees, 4 positions, 7-day template, ASSIGNING-state publication, ~hundreds of per-slot submissions sourced from the anonymized CSV.

### D-8. Test approach

- **Unit:** existing seed `main_test.go` tests the production guard. No additional unit test needed for the new scenario — it's a pure function of compiled-in data.
- **Smoke (manual + CI-friendly):** the existing tasks list calls `make seed SCENARIO=realistic` and confirms row counts: 42 + 1 (admin) users; 4 positions; 35 template_slots; 70 template_slot_positions; ~hundreds of availability_submissions; 0 assignments; 1 publication.
- **Audit silence:** seed continues to skip audit emission per the existing requirement.

## Risks / Trade-offs

- **Risk:** the `realistic.go` literal grows large and noisy in diffs. → Mitigation: 42 entries × ~5 fields each is ~250 lines; line-length tractable; one-time commit.
- **Risk:** the source CSV gets renamed / deleted, making the python script unrunnable. → Mitigation: the script is reference-only post-apply; the committed Go file is what builds and runs. Re-generation is rare.
- **Risk:** the role-archetype RNG output isn't satisfying for the user's smoke testing. → Mitigation: bumping `realisticAssignmentSeed` to a different constant + re-running the script gives a fresh sampling. The user can also hand-tweak the slice literal for specific overrides.
- **Trade-off:** the seed file imports `database/sql` and `context` like other scenarios — no new dep surface. The tradeoff with embedding vs reading a file at runtime: embedding is verbose but ships a single binary that doesn't need any companion files.

## Migration Plan

Single shipping unit. After merge, `make seed SCENARIO=realistic` is available. Existing `basic / full / stress` scenarios untouched.

Rollback = revert the change; `realistic.go` disappears, dispatcher loses the case.

## Open Questions

None.
