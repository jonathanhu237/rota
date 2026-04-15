import { isAxiosError } from "axios"

export type ApiErrorCode =
  | "EMAIL_ALREADY_EXISTS"
  | "FORBIDDEN"
  | "INVALID_HEADCOUNT"
  | "INVALID_CREDENTIALS"
  | "INVALID_REQUEST"
  | "INVALID_SHIFT_TIME"
  | "INVALID_WEEKDAY"
  | "INTERNAL_ERROR"
  | "PASSWORD_TOO_SHORT"
  | "POSITION_IN_USE"
  | "POSITION_NOT_FOUND"
  | "TEMPLATE_LOCKED"
  | "TEMPLATE_NOT_FOUND"
  | "TEMPLATE_SHIFT_NOT_FOUND"
  | "UNAUTHORIZED"
  | "USER_DISABLED"
  | "USER_NOT_FOUND"
  | "VERSION_CONFLICT"

export type ApiErrorResponse = {
  error?: {
    code?: ApiErrorCode
    message?: string
  }
}

export function getApiErrorDetails(error: unknown) {
  if (!isAxiosError<ApiErrorResponse>(error)) {
    return undefined
  }

  return error.response?.data?.error
}

export function getTranslatedApiError(
  t: (key: string, options?: Record<string, unknown>) => string,
  error: unknown,
  keyPrefix: string,
  fallbackKey: string,
) {
  const apiError = getApiErrorDetails(error)
  if (apiError?.code) {
    return t(`${keyPrefix}.${apiError.code}`, {
      defaultValue: apiError.message ?? t(fallbackKey),
    })
  }

  return apiError?.message ?? t(fallbackKey)
}
