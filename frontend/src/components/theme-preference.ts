// Split out from theme-provider.tsx so localStorage / matchMedia helpers can
// run before React mounts (the inline boot script in index.html and the
// provider both consume them) without dragging React imports into a
// Fast-Refresh-sensitive .tsx.
import type { ThemePreference } from "@/lib/types"

export const themeStorageKey = "rota:theme"

export type EffectiveTheme = "light" | "dark"

export function getStoredThemePreference(): ThemePreference {
  const storage = getLocalStorage()
  const stored = storage?.getItem(themeStorageKey)
  return isThemePreference(stored) ? stored : "system"
}

export function applyThemePreference(
  preference: ThemePreference,
  systemPrefersDark = getSystemPrefersDark(),
): EffectiveTheme {
  const effectiveTheme = getEffectiveTheme(preference, systemPrefersDark)
  getLocalStorage()?.setItem(themeStorageKey, preference)

  if (typeof document !== "undefined") {
    document.documentElement.classList.toggle("dark", effectiveTheme === "dark")
    document.documentElement.classList.toggle("light", effectiveTheme === "light")
  }

  return effectiveTheme
}

export function getEffectiveTheme(
  preference: ThemePreference,
  systemPrefersDark = getSystemPrefersDark(),
): EffectiveTheme {
  if (preference === "dark" || preference === "light") {
    return preference
  }
  return systemPrefersDark ? "dark" : "light"
}

export function getSystemPrefersDark() {
  if (typeof window === "undefined" || !window.matchMedia) {
    return false
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches
}

function isThemePreference(value: string | null | undefined): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system"
}

function getLocalStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null
  }
  if (
    !window.localStorage ||
    typeof window.localStorage.getItem !== "function" ||
    typeof window.localStorage.setItem !== "function"
  ) {
    return null
  }
  return window.localStorage
}
