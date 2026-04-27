// Split out from theme-provider.tsx so React Fast Refresh's "components-only"
// boundary keeps the provider hot-reloadable: a .tsx that exports both a
// component and a hook breaks Fast Refresh.
import { createContext, useContext } from "react"

import type { EffectiveTheme } from "@/components/theme-preference"
import type { ThemePreference } from "@/lib/types"

export type ExplicitThemePreference = Exclude<ThemePreference, "system">

export type ThemeContextValue = {
  preference: ThemePreference
  effectiveTheme: EffectiveTheme
  setThemePreference: (preference: ThemePreference) => void
  toggleThemePreference: () => ExplicitThemePreference
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

export function useTheme() {
  const context = useContext(ThemeContext)
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider")
  }
  return context
}
