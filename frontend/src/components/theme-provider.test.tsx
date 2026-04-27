import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, waitFor } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { currentUserQueryOptions } from "@/lib/queries"
import type { ThemePreference, User } from "@/lib/types"

import { themeStorageKey } from "./theme-preference"
import { ThemeProvider } from "./theme-provider"

function makeUser(themePreference: ThemePreference): User {
  return {
    id: 1,
    email: "alice@example.com",
    name: "Alice Example",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: themePreference,
  }
}

function renderThemeProvider(themePreference: ThemePreference) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(currentUserQueryOptions.queryKey, makeUser(themePreference))

  return render(
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <div>child</div>
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

function mockLocalStorage() {
  const store = new Map<string, string>()
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      getItem: vi.fn((key: string) => store.get(key) ?? null),
      setItem: vi.fn((key: string, value: string) => {
        store.set(key, value)
      }),
      removeItem: vi.fn((key: string) => {
        store.delete(key)
      }),
      clear: vi.fn(() => {
        store.clear()
      }),
    },
  })
}

describe("ThemeProvider", () => {
  beforeEach(() => {
    mockLocalStorage()
  })

  afterEach(() => {
    window.localStorage.clear()
    document.documentElement.classList.remove("dark", "light")
  })

  it("applies the dark class for a dark preference", async () => {
    mockMatchMedia(false)

    renderThemeProvider("dark")

    await waitFor(() => {
      expect(document.documentElement).toHaveClass("dark")
    })
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark")
  })

  it("uses prefers-color-scheme when preference is system", async () => {
    mockMatchMedia(true)

    renderThemeProvider("system")

    await waitFor(() => {
      expect(document.documentElement).toHaveClass("dark")
    })
    expect(window.localStorage.getItem(themeStorageKey)).toBe("system")
  })
})
