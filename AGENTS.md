## Development Workflow

This project uses **OpenSpec** for spec-driven development. All change proposals, designs, tasks, and specs live under `openspec/` and are tracked in git.

### Roles

- **Claude:** exploring, proposing, reviewing, committing
- **Codex:** implementing tasks, reporting outcomes

### Parallelism

Both Claude and Codex should maximize use of parallel agents whenever tasks are independent. Do not execute sequentially what can be done concurrently — spawn multiple agents in parallel for exploration, implementation, or review sub-tasks where there are no dependencies between them.

### Loop

```
/opsx:explore (optional) → /opsx:propose → /opsx:apply → review → /opsx:archive → commit
```

### Step-by-step

1. **Explore (optional):** `/opsx:explore` for thinking-partner mode — investigate the codebase, compare options, sketch designs. No code is written here.

2. **Propose:** `/opsx:propose <change-name>` scaffolds `openspec/changes/<name>/` with `proposal.md` (what & why), `design.md` (how), `tasks.md` (steps), and any delta `specs/`.

3. **Apply:** `/opsx:apply` reads the artifacts and works through `tasks.md`, ticking each `- [ ]` to `- [x]` as it completes. Codex usually drives this step.

4. **Review:** Claude verifies the implementation against the change artifacts. Issues are fixed by re-running `/opsx:apply` or by direct edits.

5. **Archive:** `/opsx:archive` moves the change to `openspec/changes/archive/YYYY-MM-DD-<name>/` and syncs delta specs into `openspec/specs/`. Commit the result.

Detailed step-by-step instructions live in `.claude/skills/openspec-*/SKILL.md` (and the mirror under `.codex/skills/`).

### Commit convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(scope):` new feature
- `fix(scope):` bug fix
- `chore:` tooling, config, dependencies
- `docs:` documentation only

The archived change directory under `openspec/changes/archive/` is part of the commit — that's the durable record of what was built and why.

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
- When applying a change, the implementer must report which tests were added and whether they pass.
- Reviewers must flag missing or insufficient test coverage as an issue before archiving.

---

## Code Conventions

- Write all code comments in **English**.
- In Go code: if `interface{}` is needed, use `any` instead.
- In Go code: keep SQL statement indentation aligned with the surrounding Go indentation.
