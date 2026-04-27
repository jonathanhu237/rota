import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { useQuery } from "@tanstack/react-query"

import {
  ThemeContext,
  type ExplicitThemePreference,
} from "@/components/theme-context"
import {
  applyThemePreference,
  getEffectiveTheme,
  getStoredThemePreference,
  getSystemPrefersDark,
} from "@/components/theme-preference"
import { currentUserQueryOptions } from "@/lib/queries"
import type { ThemePreference } from "@/lib/types"

export function ThemeProvider({ children }: { children: ReactNode }) {
  const { data: user } = useQuery(currentUserQueryOptions)
  const [localPreference, setLocalPreference] =
    useState<ThemePreference | null>(null)
  const [systemPrefersDark, setSystemPrefersDark] = useState(() =>
    getSystemPrefersDark(),
  )
  const preference =
    localPreference ?? user?.theme_preference ?? getStoredThemePreference()
  const effectiveTheme = getEffectiveTheme(preference, systemPrefersDark)

  useEffect(() => {
    applyThemePreference(preference, systemPrefersDark)
  }, [preference, systemPrefersDark])

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return
    }

    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const handleChange = (event: MediaQueryListEvent) => {
      setSystemPrefersDark(event.matches)
    }

    media.addEventListener("change", handleChange)
    return () => media.removeEventListener("change", handleChange)
  }, [preference])

  const toggleThemePreference = useCallback(() => {
    const nextPreference: ExplicitThemePreference =
      effectiveTheme === "dark" ? "light" : "dark"
    setLocalPreference(nextPreference)
    return nextPreference
  }, [effectiveTheme])

  const value = useMemo(
    () => ({
      preference,
      effectiveTheme,
      setThemePreference: setLocalPreference,
      toggleThemePreference,
    }),
    [effectiveTheme, preference, toggleThemePreference],
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
