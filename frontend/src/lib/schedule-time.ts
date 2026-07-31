type ScheduleDateTimeFormatOptions = Omit<
  Intl.DateTimeFormatOptions,
  "timeZone"
>

export function createScheduleDateTimeFormatter(
  locale: string | undefined,
  options: ScheduleDateTimeFormatOptions,
) {
  return new Intl.DateTimeFormat(locale, {
    ...options,
    timeZone: "UTC",
  })
}

export function scheduleDateTimeToLocalInput(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ""
  }
  return date.toISOString().slice(0, 16)
}

export function scheduleDateTimeLocalToISOString(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) {
    return ""
  }

  const date = new Date(`${value}:00.000Z`)
  if (
    Number.isNaN(date.getTime()) ||
    date.toISOString().slice(0, 16) !== value
  ) {
    return ""
  }
  return date.toISOString()
}
