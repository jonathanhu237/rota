# Hook and Data-Fetching Guidelines

## Server State

Use TanStack Query for API reads and mutations. The shared Axios client in
`src/lib/axios.ts` has `baseURL: "/api"` and credentials enabled.

Reusable reads are exported as `queryOptions` objects or factories from
`src/lib/queries.ts`:

```ts
export const userQueryOptions = (userID: number) =>
  queryOptions({
    queryKey: ["users", "detail", userID],
    queryFn: async () => {
      const res = await api.get<UserResponse>(`/users/${userID}`)
      return res.data.user
    },
    enabled: userID > 0,
  })
```

Query keys are arrays that start with a stable feature namespace. Include every
input that changes the result. Reuse the same key prefix for invalidation.
Authenticated loaders use `ensureQueryData` where route entry depends on the
result.

Mutations call typed functions from `lib/queries.ts` or a small feature-owned
mutation descriptor such as `components/settings/settings-api.ts`. On success,
either set the exact cached value or invalidate the narrow affected key.

## Error Handling

Use `getTranslatedApiError` from `src/lib/api-error.ts` for user-visible API
failures. Authentication handling may inspect Axios status to redirect or stop
retries, as in `src/routes/_authenticated.tsx` and `currentUserQueryOptions`.
Do not broadly swallow errors; the branding fallback is an intentional
bootstrap exception.

## Custom Hooks

Create a custom hook only for reusable stateful React behavior. Name it `use*`
and place truly shared hooks in `src/hooks/`; feature-specific hooks remain
beside the feature. Pure transformations belong in ordinary functions, not
hooks.

## Avoid

- Fetching API data in `useEffect`.
- Copying query results into component state.
- Query keys that omit pagination, entity IDs, or other result inputs.
- Invalidating all queries when a feature prefix is sufficient.
- A custom hook that only wraps a one-line pure function.
