# Frontend

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
