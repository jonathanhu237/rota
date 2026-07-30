# Cross-Layer Change Guide

Use this guide when a change crosses two or more of the database, backend, API,
frontend, email, audit, or operational boundaries.

## Map the Contract First

For a persisted product field or behavior, trace the complete flow:

```text
migration
  -> model
  -> repository scan/write
  -> service validation/state transition
  -> handler request/response/error code
  -> frontend shared type/API function/query key
  -> route or component
  -> en + zh translations
  -> tests at each changed boundary
```

Write the exact field name, nullability, time/date format, enum values, default,
and owner at every boundary. Rota's JSON uses snake case. Do not change casing
or optionality in only one layer.

## Boundary Checklist

### Database and Repository

- Does the migration encode the invariant with a constraint or index?
- Do scans and writes include the new field in every relevant query?
- Is a multi-write transition protected by one transaction and any required
  row locks?
- Are date-only values normalized before comparison?

### Service and Domain

- Which layer owns validation and authorization?
- Does the change preserve publication, template, assignment, attendance, and
  shift-change state invariants?
- Is time injected through the existing clock boundary?
- Are expected failures stable sentinel errors?
- Does the mutation need an audit event or transactional outbox message?

### HTTP and Frontend

- Do request/response DTOs and `frontend/src/lib/types.ts` agree exactly?
- Is each new API code present in `frontend/src/lib/api-error.ts` and both
  locale files?
- Does TanStack Query use a key containing every result input and invalidate
  the narrow affected cache?
- Are loading, empty, rejection, and success states visible and translated?

### Operations

- Is every new environment variable documented in `.env.example`?
- Does a sensitive value remain blank and outside Git?
- Does the deployment or worker need a compatibility/rollback sequence?

## High-Risk Rota Boundaries

- Publication lifecycle:
  `DRAFT -> COLLECTING -> ASSIGNING -> PUBLISHED -> ACTIVE -> ENDED`.
- Only one non-ended publication may exist.
- Referenced templates are immutable.
- Occurrence overrides must survive roster reads and attendance checks.
- Shift-change approval uses locking and can invalidate related pending
  requests.
- Email notification state and its outbox row commit atomically.

Load the matching file under `../domain/` before changing any of these.

## Verification

Check more than compilation:

- a success path and an expected rejection/error path;
- a round trip through every changed serialization boundary;
- empty, null, Unicode, timezone, and concurrency cases that apply;
- integration tests for SQL;
- both locale trees and every consumer found with `rg`.

Every review finding must be verified against actual code and the domain
contract before it becomes a required change.
