import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import en from "./locales/en.json"
import zh from "./locales/zh.json"

export const languageStorageKey = "rota:lang"
const legacyLanguageStorageKey = "rota-language"

export function normalizeLanguage(language?: string | null) {
  if (!language) {
    return "zh"
  }

  return language.toLowerCase().startsWith("zh") ? "zh" : "en"
}

function getInitialLanguage() {
  if (typeof window === "undefined") {
    return "en"
  }

  const storage = getLanguageStorage()
  const storedLanguage =
    storage?.getItem(languageStorageKey) ??
    storage?.getItem(legacyLanguageStorageKey)
  if (storedLanguage === "en" || storedLanguage === "zh") {
    return storedLanguage
  }

  return normalizeLanguage(window.navigator.language)
}

function syncDocumentLanguage(language: string) {
  if (typeof document !== "undefined") {
    document.documentElement.lang = normalizeLanguage(language)
  }
}

const initialLanguage = getInitialLanguage()
syncDocumentLanguage(initialLanguage)

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  lng: initialLanguage,
  fallbackLng: "en",
  supportedLngs: ["en", "zh"],
  interpolation: {
    escapeValue: false,
  },
})

if (typeof window !== "undefined") {
  i18n.on("languageChanged", (language) => {
    const normalizedLanguage = normalizeLanguage(language)
    getLanguageStorage()?.setItem(languageStorageKey, normalizedLanguage)
    syncDocumentLanguage(normalizedLanguage)
  })
}

export function applyLanguagePreference(language: "zh" | "en") {
  getLanguageStorage()?.setItem(languageStorageKey, language)
  return i18n.changeLanguage(language)
}

function getLanguageStorage() {
  if (typeof window === "undefined") {
    return undefined
  }

  try {
    const storage = window.localStorage
    if (
      !storage ||
      typeof storage.getItem !== "function" ||
      typeof storage.setItem !== "function"
    ) {
      return undefined
    }
    return storage
  } catch {
    return undefined
  }
}

export default i18n
