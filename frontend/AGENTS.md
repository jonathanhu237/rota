# Frontend

## Style guide references

The mechanical layer is enforced by `tsc --strict` and the project's ESLint flat config — run `pnpm lint` and `pnpm tsc --noEmit`. Beyond that, no single style guide is mandatory; consult these when an idiomatic question arises:

- [React docs](https://react.dev) — the canonical reference for hooks, suspense, and component patterns. Prefer it over older third-party React style guides written for class components.
- [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html) — for type-system questions.
- [TanStack Query docs](https://tanstack.com/query/latest) — for query/mutation patterns we use across the app.

Project conventions below override these where they conflict. When local files already establish a pattern, follow that pattern over starting a new one.

## Stack

- React 19 + Vite, TypeScript.
- UI: shadcn/ui on top of Tailwind v4.
- Data: TanStack Query.
- Routing: TanStack Router (file-based).
- Forms and validation: React Hook Form + Zod (pinned to `zod/v3` — **not** v4).
- HTTP: Axios.
- i18n: i18next + react-i18next. Locales: `en` and `zh`.

## Conventions

- Every user-facing string goes through i18next. No hardcoded UI text, including toasts, badges, and error messages.
- Tests are pure-logic (schemas, helpers, query builders) unless a rendering test is explicitly scoped. Tests live beside the code they cover.
- Components put form state in React Hook Form; don't manage form values in local `useState`.
- Don't import from `zod`. Import from `zod/v3`.
