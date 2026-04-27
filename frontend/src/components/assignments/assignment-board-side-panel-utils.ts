import { formatHours } from "@/components/assignments/draft-state"

export const weekdayKeys: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

export function formatUserLabel(
  t: (key: string, options?: Record<string, unknown>) => string,
  name: string,
  hours: number,
) {
  const formattedHours = formatHours(hours)
  const translated = t("assignments.drafts.userHoursLabel", {
    user: name,
    hours: formattedHours,
  })

  return translated === "assignments.drafts.userHoursLabel"
    ? `${name} (${formattedHours}h)`
    : translated
}
