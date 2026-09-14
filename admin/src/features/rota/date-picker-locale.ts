import { enUS, zhCN } from "react-day-picker/locale"

export function normalizeLanguage(language?: string | null): "en" | "zh" {
  return language?.toLowerCase().startsWith("zh") ? "zh" : "en"
}

export function getDatePickerLocale(language?: string | null) {
  return normalizeLanguage(language) === "zh" ? zhCN : enUS
}

export function formatDatePickerDisplayDate(
  date: Date,
  language?: string | null,
) {
  return new Intl.DateTimeFormat(normalizeLanguage(language), {
    dateStyle: "medium",
  }).format(date)
}
