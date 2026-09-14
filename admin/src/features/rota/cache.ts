import type { Query, QueryClient } from "@tanstack/react-query"

// Rota query keys predate the template app's auth cache and therefore do not
// all share one namespace. Keep the allow-list here as the protected-cache
// contract so a session boundary cannot accidentally leave a private Rota
// query behind when a new feature adds another top-level key.
const protectedRotaRoots = new Set([
  "users",
  "positions",
  "templates",
  "publications",
  "roster",
  "attendance",
  "leaves",
  "me",
  "rota",
  "auth",
])

export function isProtectedRotaQuery(query: Query): boolean {
  const root = query.queryKey[0]
  return typeof root === "string" && protectedRotaRoots.has(root)
}

/**
 * Cancel and remove every Rota query that may contain account-scoped data.
 * This is intentionally a predicate rather than a handful of prefix calls:
 * list/detail keys use several historical shapes and all must cross the same
 * logout, auth-error, and account-switch boundary.
 */
export async function clearProtectedRotaCache(queryClient: QueryClient): Promise<void> {
  const protectedQueries = { predicate: isProtectedRotaQuery }
  // Remove first so a principal transition cannot render the old value while
  // cancellation waits for a delayed adapter. Removing again after cancel
  // also covers adapters that resolve a non-cooperative promise late.
  const cancellation = queryClient.cancelQueries(protectedQueries)
  queryClient.removeQueries(protectedQueries)
  await cancellation
  queryClient.removeQueries(protectedQueries)
}
