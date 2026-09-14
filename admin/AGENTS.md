# Admin

## Stack

React 19 + Vite + TypeScript, shadcn/ui and Tailwind v4, TanStack Query and
TanStack Router, React Hook Form + Zod, Axios, and i18next (`en` and `zh-CN`).

## Conventions

- Every user-facing string goes through i18next, including toasts, badges, and
  error messages.
- Prefer pure-logic tests for schemas, helpers, query builders, and cache
  lifecycle contracts; add rendering tests for critical navigation boundaries.
- Keep protected query data behind the session/account cache contract in
  `features/rota/cache.ts` and clear it on logout, auth failure, and account
  changes.
- Use `pnpm lint`, `pnpm check`, `pnpm test`, and `pnpm build` before completion.
