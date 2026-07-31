import { describe, expect, it } from "vitest"

import {
  createScheduleDateTimeFormatter,
  scheduleDateTimeLocalToISOString,
  scheduleDateTimeToLocalInput,
} from "./schedule-time"

describe("schedule wall-clock time", () => {
  it("formats UTC clock fields without applying the browser timezone", () => {
    const formatter = createScheduleDateTimeFormatter("en-US", {
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    })
    const parts = formatter.formatToParts(
      new Date("2026-08-03T08:00:00Z"),
    )

    expect(formatter.resolvedOptions().timeZone).toBe("UTC")
    expect(parts.find((part) => part.type === "hour")?.value).toBe("08")
    expect(parts.find((part) => part.type === "minute")?.value).toBe("00")
  })

  it("round-trips datetime-local values through UTC without an offset shift", () => {
    const input = scheduleDateTimeToLocalInput("2026-08-03T08:00:00Z")

    expect(input).toBe("2026-08-03T08:00")
    expect(scheduleDateTimeLocalToISOString(input)).toBe(
      "2026-08-03T08:00:00.000Z",
    )
  })

  it("rejects invalid schedule input values", () => {
    expect(scheduleDateTimeToLocalInput("not-a-date")).toBe("")
    expect(scheduleDateTimeLocalToISOString("2026-02-30T08:00")).toBe("")
  })
})
