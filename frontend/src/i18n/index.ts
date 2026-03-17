import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import en from "./locales/en.json"
import zh from "./locales/zh.json"

const languageStorageKey = "rota-language"

function normalizeLanguage(language?: string | null) {
  if (!language) {
    return "en"
  }

  return language.toLowerCase().startsWith("zh") ? "zh" : "en"
}

function getInitialLanguage() {
  if (typeof window === "undefined") {
    return "en"
  }

  const storedLanguage = window.localStorage.getItem(languageStorageKey)
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
    window.localStorage.setItem(
      languageStorageKey,
      normalizeLanguage(language),
    )
  })
}

export default i18n
