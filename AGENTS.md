## Development Workflow

This project uses a structured Claude + Codex collaboration loop. All participants must follow this workflow.

### Roles

- **Claude:** planning, code review, committing
- **Codex:** implementation, execution summary

### Parallelism

Both Claude and Codex should maximize use of parallel agents whenever tasks are independent. Do not execute sequentially what can be done concurrently — spawn multiple agents in parallel for exploration, implementation, or review sub-tasks where there are no dependencies between them.

### Loop

```
Claude → PLAN.md → Codex → SUMMARY.md → Claude → REVIEW.md → Codex (fix) → Claude (verify) → commit
```

### Step-by-step

1. **Claude writes `PLAN.md`** to the project root before any implementation begins.
   - Must include: context, goal, file-level change list, verification steps.
   - Describe **intent and constraints**, not implementation details. Do not paste code snippets into the plan — let Codex decide how to implement. Overly prescriptive plans cause Codex to copy-paste rather than reason.

2. **Codex implements** according to `PLAN.md`, then writes `SUMMARY.md` to the project root.
   - `SUMMARY.md` must cover: what was done, what was verified, any blockers or deviations from the plan.

3. **Claude reviews** the implementation against `PLAN.md` and `SUMMARY.md`, then writes `REVIEW.md` to the project root.
   - `REVIEW.md` must include: verdict (LGTM / issues found), what Codex did well, and each issue with file + line reference and a concrete fix.

4. **If issues exist:** Codex reads `REVIEW.md` and fixes all items. Return to step 3.

5. **If LGTM:** Claude verifies the final state, deletes `PLAN.md`, `REVIEW.md`, and `SUMMARY.md`, then creates a conventional commit.

### Commit convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(scope):` new feature
- `fix(scope):` bug fix
- `chore:` tooling, config, dependencies
- `docs:` documentation only

Do not commit intermediate files (`PLAN.md`, `REVIEW.md`, `SUMMARY.md`).

---

## Project Overview

This project is a web-based **shift scheduling system** for departments.

Consider a department with ~100 employees and multiple positions. Positions are not fixed — e.g., Position A might be staffed by Employee X on Monday morning and Employee Y on Tuesday morning, while each employee has their own availability constraints.

The system allows employees to submit their available time slots, and administrators to create schedules based on that data. It also provides algorithmic auto-scheduling to reduce manual effort.

## Core Features

- **Employee Availability**
  Employees submit their free time slots so the system knows when each person can work.

- **Admin Scheduling**
  Administrators view availability data and manually assign employees to positions and time slots.

- **Auto-Scheduling**
  The system uses scheduling algorithms to generate optimized rosters automatically, which admins can then review and adjust.

- **Multi-Position Support**
  Departments can define multiple positions, each with its own staffing requirements and rotation rules.

---

## Tech Stack

### Frontend

- **Framework:** React 19 + Vite
- **Routing:** TanStack Router (file-based)
- **State management:** TanStack Query
- **Forms:** React Hook Form + Zod (use `zod/v3`)
- **UI:** shadcn/ui (Tailwind CSS v4)
- **HTTP client:** Axios
- **i18n:** i18next + react-i18next
  - Supported languages: English (`en`) and Chinese (`zh`)
  - Default language: follows browser locale, fallback to `en`
  - All user-facing strings must go through i18next — no hardcoded UI text
- **Package manager:** pnpm

### Backend

- **Language:** Go
- **Database:** PostgreSQL 17, launched via `docker-compose.yml`
- **Cache / Session:** Redis 8, launched via `docker-compose.yml`
- **Migrations:** goose (SQL-based, stored in `migrations/`)
- **Config:** `caarlos0/env` (environment variables)

### Infrastructure

- **Orchestration:** Docker Compose
- **Build / Dev tasks:** Makefile (`run-backend`, `run-frontend`, `migrate-up`, `migrate-down`, `migrate-status`)

---

## Config

- Configuration is managed exclusively via environment variables.
- New config items must be added to `.env.example`. Provide default values for non-sensitive items; leave sensitive items blank.
- Update `.env` accordingly when adding new config items.
- For local development passwords, use `pa55word` uniformly.

---

## Testing

- Every new feature or bug fix **must** include tests.
- Backend: use Go's standard `testing` package. Place test files alongside the code they test (e.g., `user_test.go` next to `user.go`).
- Frontend: use Vitest for unit tests.
- Tests should cover the main success path and key error cases. Do not skip tests just because the feature "seems simple."
- The `SUMMARY.md` from Codex must report which tests were added and whether they pass.
- Claude's `REVIEW.md` must flag missing or insufficient test coverage as an issue.

---

## Code Conventions

- Write all code comments in **English**.
- In Go code: if `interface{}` is needed, use `any` instead.
- In Go code: keep SQL statement indentation aligned with the surrounding Go indentation.
