# Frontend Development Guidelines

The frontend is React 19 and TypeScript on Vite. It uses TanStack Router,
TanStack Query, Axios, shadcn/ui with Tailwind v4, React Hook Form, `zod/v3`,
and i18next (`en` and `zh`).

## Guidelines Index

| Guide | Description |
|---|---|
| [Directory Structure](./directory-structure.md) | Routes, feature components, shared libraries, and generated files |
| [Component Guidelines](./component-guidelines.md) | Composition, forms, styling, accessibility, and i18n |
| [Hook Guidelines](./hook-guidelines.md) | Query options, mutations, and custom hooks |
| [State Management](./state-management.md) | Server, URL, form, context, and local state |
| [Type Safety](./type-safety.md) | API types, schemas, and strict TypeScript |
| [Quality Guidelines](./quality-guidelines.md) | Tests, lint/build checks, and forbidden patterns |

## Pre-Development Checklist

Read `quality-guidelines.md` and then:

- New or changed UI: `component-guidelines.md`.
- Route or navigation work: `directory-structure.md` and
  `state-management.md`.
- Data fetching or mutation work: `hook-guidelines.md`,
  `state-management.md`, and `type-safety.md`.
- Forms or validation: `component-guidelines.md` and `type-safety.md`.
- Backend/frontend contract changes: also read
  `../guides/cross-layer-thinking-guide.md` and the matching `../domain/` spec.

Search for an established feature pattern before creating a new component,
query key, schema helper, or shared utility.

## Non-Negotiable Conventions

- Every user-facing string, including errors and toasts, goes through i18next.
- Forms use React Hook Form; do not mirror form fields in local `useState`.
- Import Zod from `zod/v3`, never from `zod`.
- Server data is owned by TanStack Query rather than copied into local state.
- API error codes remain synchronized with backend handler mappings.

## Quality Check

- Run `pnpm lint`, `pnpm test`, and `pnpm build` from `frontend/` on
  Centaurus.
- Confirm both locale files contain every new user-facing key.
- Confirm API fields/error codes match shared frontend types and backend
  responses.
- Confirm query keys and invalidation cover every changed result input.
- Exercise success plus rejection/error/empty behavior appropriate to the UI
  change.
