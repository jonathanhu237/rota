import { isAxiosError } from "axios"

export type ApiErrorCode =
  | "EMAIL_ALREADY_EXISTS"
  | "FORBIDDEN"
  | "INVALID_CREDENTIALS"
  | "INVALID_REQUEST"
  | "INTERNAL_ERROR"
  | "PASSWORD_TOO_SHORT"
  | "POSITION_NOT_FOUND"
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
