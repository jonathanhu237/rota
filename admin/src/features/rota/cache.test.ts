import { QueryClient } from "@tanstack/react-query"
import { describe, expect, it } from "vitest"
import { clearProtectedRotaCache } from "./cache"

describe("Rota protected cache lifecycle", () => {
  it("removes private Rota data while retaining unrelated public shell data", async () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(["auth", "current-user"], { id: "user-a" })
    queryClient.setQueryData(["me", "leaves", 1, 10], { leaves: ["private-a"] })
    queryClient.setQueryData(["publications", "current"], { id: 12 })
    queryClient.setQueryData(["shell", "current-user"], { id: "user-a" })
    queryClient.setQueryData(["setup", "status"], { status: "ready" })

    await clearProtectedRotaCache(queryClient)

    expect(queryClient.getQueryData(["auth", "current-user"])).toBeUndefined()
    expect(queryClient.getQueryData(["me", "leaves", 1, 10])).toBeUndefined()
    expect(queryClient.getQueryData(["publications", "current"])).toBeUndefined()
    expect(queryClient.getQueryData(["shell", "current-user"])).toEqual({ id: "user-a" })
    expect(queryClient.getQueryData(["setup", "status"])).toEqual({ status: "ready" })
  })

  it("clears every historical protected key shape, including in-flight failures", async () => {
    const queryClient = new QueryClient()
    const never = new Promise<never>(() => undefined)
    void queryClient.prefetchQuery({ queryKey: ["attendance", "current"], queryFn: () => never })
    queryClient.setQueryData(["leaves", "pool", "pending", 1, 20], { leaves: ["private-a"] })
    queryClient.setQueryData(["templates", "detail", 4], { id: 4 })

    await clearProtectedRotaCache(queryClient)

    expect(queryClient.getQueryData(["attendance", "current"])).toBeUndefined()
    expect(queryClient.getQueryData(["leaves", "pool", "pending", 1, 20])).toBeUndefined()
    expect(queryClient.getQueryData(["templates", "detail", 4])).toBeUndefined()
  })

  it("does not let a delayed response repopulate a cleared account cache", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    let resolveRequest!: (value: { owner: string }) => void
    const request = queryClient.fetchQuery({
      queryKey: ["publications", "detail", 7],
      queryFn: ({ signal }) => new Promise<{ owner: string }>((resolve, reject) => {
        resolveRequest = resolve
        signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true })
      }),
    }).catch(() => undefined)

    await clearProtectedRotaCache(queryClient)
    expect(queryClient.getQueryData(["publications", "detail", 7])).toBeUndefined()

    resolveRequest({ owner: "account-a" })
    await request
    expect(queryClient.getQueryData(["publications", "detail", 7])).toBeUndefined()
  })
})
