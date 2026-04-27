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

  const storedLanguage =
    window.localStorage.getItem(languageStorageKey) ??
    window.localStorage.getItem(legacyLanguageStorageKey)
  if (storedLanguage === "en" || storedLanguage === "zh") {
    return storedLanguage
  }

  return normalizeLanguage(window.navigator.language)
}

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  lng: getInitialLanguage(),
  fallbackLng: "en",
  supportedLngs: ["en", "zh"],
  interpolation: {
    escapeValue: false,
  },
})

if (typeof window !== "undefined") {
  i18n.on("languageChanged", (language) => {
    window.localStorage.setItem(languageStorageKey, normalizeLanguage(language))
  })
}

export function applyLanguagePreference(language: "zh" | "en") {
  window.localStorage.setItem(languageStorageKey, language)
  return i18n.changeLanguage(language)
}

export default i18n
