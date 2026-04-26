import { describe, expect, it, vi } from "vitest"

import {
  getApiErrorDetails,
  getTranslatedApiError,
  type ApiErrorResponse,
} from "./api-error"

function createAxiosLikeError(data?: ApiErrorResponse) {
  return {
    isAxiosError: true,
    response: data ? { data } : undefined,
  }
}

describe("getApiErrorDetails", () => {
  it("returns undefined for non-Axios errors", () => {
    expect(getApiErrorDetails(new Error("boom"))).toBeUndefined()
  })

  it("extracts error code from Axios error response", () => {
    const details = getApiErrorDetails(
      createAxiosLikeError({
        error: {
          code: "INVALID_REQUEST",
          message: "Invalid request",
        },
      }),
    )

    expect(details).toEqual({
      code: "INVALID_REQUEST",
      message: "Invalid request",
    })
  })

  it("extracts leave error codes", () => {
    const details = getApiErrorDetails(
      createAxiosLikeError({
        error: {
          code: "LEAVE_NOT_OWNER",
          message: "Not authorized for this leave",
        },
      }),
    )

    expect(details?.code).toBe("LEAVE_NOT_OWNER")
  })
})

describe("getTranslatedApiError", () => {
  it("translates error code with the provided key prefix", () => {
    const t = vi.fn((key: string, options?: Record<string, unknown>) => {
      return `${key}:${String(options?.defaultValue ?? "")}`
    })

    const message = getTranslatedApiError(
      t,
      createAxiosLikeError({
        error: {
          code: "USER_NOT_FOUND",
          message: "User not found",
        },
      }),
      "users.errors",
      "users.errors.INTERNAL_ERROR",
    )

    expect(message).toBe("users.errors.USER_NOT_FOUND:User not found")
    expect(t).toHaveBeenCalledTimes(1)
    expect(t).toHaveBeenCalledWith("users.errors.USER_NOT_FOUND", {
      defaultValue: "User not found",
    })
  })

  it("falls back to the API message when no error code is present", () => {
    const t = vi.fn((key: string) => key)

    const message = getTranslatedApiError(
      t,
      createAxiosLikeError({
        error: {
          message: "Raw API message",
        },
      }),
      "users.errors",
      "users.errors.INTERNAL_ERROR",
    )

    expect(message).toBe("Raw API message")
    expect(t).not.toHaveBeenCalled()
  })

  it("falls back to the translated fallback key when there is no API error payload", () => {
    const t = vi.fn((key: string) => `translated:${key}`)

    const message = getTranslatedApiError(
      t,
      createAxiosLikeError(),
      "users.errors",
      "users.errors.INTERNAL_ERROR",
    )

    expect(message).toBe("translated:users.errors.INTERNAL_ERROR")
    expect(t).toHaveBeenCalledWith("users.errors.INTERNAL_ERROR")
  })
})
