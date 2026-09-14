import api from "./axios"
import type { LanguagePreference, Publication, PublicationState } from "./types"

export function getPublicationStateTranslationKey(state: PublicationState) {
  switch (state) {
    case "DRAFT":
      return "publications.state.draft"
    case "COLLECTING":
      return "publications.state.collecting"
    case "ASSIGNING":
      return "publications.state.assigning"
    case "PUBLISHED":
      return "publications.state.published"
    case "ACTIVE":
      return "publications.state.active"
    case "ENDED":
      return "publications.state.ended"
  }
}

export type PublicationLifecycleAction = "publish" | "activate" | "end"

export function getPublicationLifecycleAction(
  state: PublicationState,
): PublicationLifecycleAction | null {
  switch (state) {
    case "ASSIGNING":
      return "publish"
    case "PUBLISHED":
      return "activate"
    case "ACTIVE":
      return "end"
    default:
      return null
  }
}

export async function fetchPublicationScheduleXLSX(
  publicationID: number,
  language: LanguagePreference,
) {
  const res = await api.get<Blob>(`/publications/${publicationID}/schedule.xlsx`, {
    params: {
      lang: language,
    },
    responseType: "blob",
  })
  return res.data
}

export async function downloadPublicationScheduleXLSX(
  publication: Publication,
  language: LanguagePreference,
  now = new Date(),
) {
  const workbook = await fetchPublicationScheduleXLSX(publication.id, language)
  saveBlobAsFile(
    workbook,
    formatScheduleExportFilename(publication.name, language, now),
  )
}

export function normalizeScheduleExportLanguage(
  language?: string | null,
): LanguagePreference {
  return language?.toLowerCase().startsWith("zh") ? "zh" : "en"
}

export function formatScheduleExportFilename(
  publicationName: string,
  language: LanguagePreference,
  date: Date,
) {
  const rosterLabel = language === "zh" ? "排班表" : "roster"
  return [
    sanitizeFilenamePart(publicationName),
    sanitizeFilenamePart(rosterLabel),
    formatLocalTimestamp(date),
  ].join("-") + ".xlsx"
}

export function saveBlobAsFile(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = filename
  document.body.append(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
}

function sanitizeFilenamePart(value: string) {
  const sanitized = value
    .trim()
    .split("")
    .map((char) => (isUnsafeFilenameChar(char) ? "-" : char))
    .join("")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "")

  return sanitized || "roster"
}

function isUnsafeFilenameChar(char: string) {
  return char.charCodeAt(0) < 32 || '<>:"/\\|?*'.includes(char)
}

function formatLocalTimestamp(date: Date) {
  return [
    padNumber(date.getFullYear(), 4),
    padNumber(date.getMonth() + 1, 2),
    padNumber(date.getDate(), 2),
  ].join("") + "-" + [
    padNumber(date.getHours(), 2),
    padNumber(date.getMinutes(), 2),
  ].join("")
}

function padNumber(value: number, length: number) {
  return value.toString().padStart(length, "0")
}

export type { PublicationState } from "./types"
