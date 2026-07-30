# Rota Domain Contracts

This directory is the authoritative source for Rota's product behavior,
interfaces, permissions, state transitions, persistence semantics, and
operational invariants.

The initial contracts were migrated on 2026-07-30 from the eight specifications
under `openspec/specs/` at base commit `158b56d`. Their Requirement/Scenario
structure was preserved. OpenSpec remains read-only legacy history; future
behavior changes update these Trellis contracts through the active task.

## Contract Index

| Contract | Scope |
|---|---|
| [Attendance](./attendance.md) | Arrival, overtime, roster snapshots, permissions, and attendance UI |
| [Audit](./audit.md) | Append-only audit taxonomy, actor context, metadata, and retention |
| [Authentication](./auth.md) | Users, sessions, invitations, password/email flows, preferences, and authorization |
| [Branding](./branding.md) | Application-wide product and organization branding |
| [Development Tooling](./dev-tooling.md) | Deployment TLS, integration-test infrastructure, and development seeding |
| [Frontend Shell](./frontend-shell.md) | Authenticated navigation, dashboard, shared page patterns, and feature workbenches |
| [Outbox](./outbox.md) | Transactional email intents, delivery worker, rendering, and configuration |
| [Scheduling](./scheduling.md) | Positions, templates, publications, availability, assignments, roster, leave, and shift changes |

## How to Use These Contracts

- Load every contract touched by a material change before designing or editing.
- Treat each `### Requirement` as normative and each `#### Scenario` as an
  executable acceptance example.
- Put task-specific deltas and decisions in the active task's `prd.md` and
  `design.md`; update the authoritative contract in the same task once the
  final behavior is known.
- If code and a contract disagree materially, resolve the artifact first rather
  than silently treating current code as the new behavior.
- Preserve stable API error codes, permission boundaries, state names, date/time
  semantics, and transaction invariants across backend and frontend changes.

## Cross-Contract Changes

Common combinations include:

- scheduling + audit for state-changing roster operations;
- auth + outbox + branding for user-facing email flows;
- scheduling + frontend-shell for employee/admin workflow changes;
- attendance + scheduling for occurrence roster and override semantics;
- dev-tooling plus any contract that adds SQL or deployment configuration.

Also read `../guides/cross-layer-thinking-guide.md` for changes that cross
database, API, frontend, email, or operational boundaries.

## Quality Check

- Confirm every material behavior change is represented in the active task and
  the affected contract before completion.
- Search the changed contract for every impacted Requirement and Scenario; do
  not update only one example of a repeated invariant.
- Trace changed fields, states, permissions, errors, and dates through the
  backend/frontend layer indexes.
- Verify code tests cover the contract's success and rejection/error scenarios.
- Preserve historical OpenSpec files; update only the Trellis contract after
  the migration baseline.
