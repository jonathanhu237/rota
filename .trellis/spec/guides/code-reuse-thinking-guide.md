# Code Reuse Guide

Search before creating a helper, contract, component, query key, error code, or
state transition. Reuse the existing owner when one already exists.

## Search First

Use `rg` from the repository root:

```bash
rg "value_or_field_name" backend frontend migrations
rg "queryKey|invalidateQueries" frontend/src
rg "Err[A-Z]|writeError" backend/internal
rg "translation.key" frontend/src/i18n/locales
```

Search by behavior and by literal value. A rename or enum addition often has
consumers in tests, translations, emails, and OpenSpec legacy history as well
as production code.

## Existing Owners

| Concern | Existing owner |
|---|---|
| Shared backend domain values/errors | `backend/internal/model/` |
| Business transitions | `backend/internal/service/` |
| PostgreSQL access | `backend/internal/repository/` |
| HTTP response/error shape | `backend/internal/handler/response.go` |
| Frontend API client/functions/query options | `frontend/src/lib/axios.ts`, `queries.ts` |
| Shared frontend API types | `frontend/src/lib/types.ts` |
| API error decoding | `frontend/src/lib/api-error.ts` |
| Form validation | Feature `*-schemas.ts` files |
| UI primitives | `frontend/src/components/ui/` |
| Product translations | Both locale JSON files |
| Behavior contracts | `.trellis/spec/domain/` |

Extend these owners instead of creating private copies in a route, component,
or adjacent service.

## When to Extract

Extract when the same non-trivial behavior has multiple consumers, when one
contract is being parsed in more than one place, or when a state transition
must remain exhaustive. Keep a one-use, obvious operation local when a shared
abstraction would hide ownership.

Good existing examples:

- `frontend/src/lib/api-error.ts` centralizes Axios error projection.
- `frontend/src/components/assignments/draft-state.ts` owns complex assignment
  draft transitions.
- model sentinel errors let repository, service, and handler share identity
  without string matching.
- narrow service-side repository interfaces reuse behavior without coupling to
  a large concrete repository.

## Avoid False Reuse

- Do not move product behavior into a generic shadcn primitive.
- Do not merge two domain operations merely because their request structs look
  similar.
- Do not build a generic repository abstraction over a few straightforward SQL
  methods.
- Do not share mutable query data through module globals.
- Do not copy backend validation into the frontend as the authoritative rule;
  frontend validation improves UX, while the service remains authoritative.

## Before Finishing

- Search for every old value and every new value.
- Confirm constants, error codes, query keys, and translations have one clear
  owner.
- Confirm copied code is either intentionally local or extracted.
- Confirm tests exercise the shared owner rather than duplicating its logic.
