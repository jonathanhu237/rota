# Frontend Quality Guidelines

## Required Patterns

- Follow the established React/TanStack patterns before introducing a new
  abstraction.
- Route every user-facing string through i18next and update both locales.
- Use React Hook Form for forms and `zod/v3` for validation.
- Keep server state in TanStack Query and API calls on the shared Axios client.
- Preserve semantic markup, labels, keyboard behavior, and accessible names.
- Keep tests beside source files.

## Testing

Prefer pure-logic tests for schemas, query builders, reducers, and helpers.
Add rendering tests when the behavior under change is inherently interactive or
provider-driven. Reuse `src/test-utils/render.tsx` and established mocks.

Tests should prove behavior at a meaningful boundary. New user-visible behavior
normally includes a success path and a rejection/error/empty path. Avoid
tautological tests that copy the implementation into the assertion.

## Quality Gate

Run from `frontend/`:

```bash
pnpm lint
pnpm test
pnpm build
```

`pnpm build` includes TypeScript compilation. Under the Centaurus workflow,
install the locked dependencies and run these checks on Centaurus after
one-way rsync from the local source-of-truth repository.

The current Centaurus validation toolchain uses pnpm 10.34.5 with the v9
lockfile. pnpm 11.14.0 exits during install because its stricter build-approval
policy treats the existing `esbuild` and `msw` lifecycle scripts as unapproved.
Do not add approval metadata as an incidental workaround; adopt pnpm 11 only in
a focused tooling task that reviews and commits the intended policy.

## Forbidden Patterns

- Hardcoded UI strings, including toast and aria text.
- Form values duplicated in local state.
- Fetching server data in `useEffect`.
- Hand-editing `src/routeTree.gen.ts`.
- Importing from `zod` rather than `zod/v3`.
- Type assertions that hide a backend/frontend contract mismatch.
- Editing generated or vendored shadcn code without checking all consumers.

## Review Checklist

- Query keys and invalidation cover every changed input/entity.
- Loading, empty, success, and error states remain coherent.
- Backend fields and API error codes match shared frontend types.
- Both locale files contain the same new key paths.
- Tests cover the behavior rather than component implementation details.
- `pnpm lint`, `pnpm test`, and `pnpm build` pass.
