# State Management

Rota does not use a general global state library. Put state in the owner that
already models its lifecycle.

## State Categories

| State | Owner |
|---|---|
| Server data, loading, retries, cache | TanStack Query |
| Route identity, params, navigation | TanStack Router |
| Form values and validation | React Hook Form + Zod |
| Theme and provider-wide UI preferences | Existing React context/provider |
| Open dialog, selection, transient drafts | Local component state |
| Derived view data | Pure functions or `useMemo` when computation warrants it |

The query client is the single client-side source of truth for server entities.
After a mutation, update or invalidate that cache rather than maintaining a
second entity copy.

## Local State

Local `useState` is appropriate for ephemeral UI state, such as the draft maps
in `src/routes/_authenticated/attendance.tsx` or open/closed controls. Key
per-row drafts by a stable composite identity rather than array index.

Complex pure transitions should be extracted into feature helpers or a reducer.
The assignment board's `draft-state.ts` and related tests are the example for
non-trivial derived interaction state.

## Context

Add React context only for cross-tree state with a clear provider lifecycle.
Follow the split used by `theme-provider.tsx` and `theme-context.ts`; do not
promote a feature's transient state globally for convenience.

## Common Mistakes

- Mirroring query data in `useState`, then drifting after invalidation.
- Using local state for React Hook Form fields.
- Storing a value that can be derived cheaply from props/query data.
- Keying state by list position when rows can reorder.
- Introducing a global store before exhausting query, router, form, context,
  and local ownership.
