import { isAxiosError } from "axios"

export type ApiErrorCode =
  | "EMAIL_ALREADY_EXISTS"
  | "FORBIDDEN"
  | "INVALID_HEADCOUNT"
  | "INVALID_OCCURRENCE_DATE"
  | "INVALID_PUBLICATION_WINDOW"
  | "INVALID_CREDENTIALS"
  | "INVALID_CURRENT_PASSWORD"
  | "INVALID_REQUEST"
  | "INVALID_SHIFT_TIME"
  | "INVALID_WEEKDAY"
  | "INTERNAL_ERROR"
  | "LEAVE_NOT_FOUND"
  | "LEAVE_NOT_OWNER"
  | "NOT_QUALIFIED"
  | "PASSWORD_TOO_SHORT"
  | "ASSIGNMENT_USER_ALREADY_IN_SLOT"
  | "ASSIGNMENT_TIME_CONFLICT"
  | "PUBLICATION_ALREADY_EXISTS"
  | "PUBLICATION_NOT_ACTIVE"
  | "PUBLICATION_NOT_ASSIGNING"
  | "PUBLICATION_NOT_COLLECTING"
  | "PUBLICATION_NOT_DELETABLE"
  | "PUBLICATION_NOT_FOUND"
  | "PUBLICATION_NOT_MUTABLE"
  | "PUBLICATION_NOT_PUBLISHED"
  | "POSITION_IN_USE"
  | "SCHEDULING_RETRYABLE"
  | "SHIFT_CHANGE_EXPIRED"
  | "SHIFT_CHANGE_INVALIDATED"
  | "SHIFT_CHANGE_INVALID_TYPE"
  | "SHIFT_CHANGE_NOT_FOUND"
  | "SHIFT_CHANGE_NOT_OWNER"
  | "SHIFT_CHANGE_NOT_PENDING"
  | "SHIFT_CHANGE_NOT_QUALIFIED"
  | "SHIFT_CHANGE_SELF"
  | "SHIFT_CHANGE_TIME_CONFLICT"
  | "POSITION_NOT_FOUND"
  | "TEMPLATE_LOCKED"
  | "TEMPLATE_NOT_FOUND"
  | "TEMPLATE_SLOT_NOT_FOUND"
  | "TEMPLATE_SLOT_OVERLAP"
  | "TEMPLATE_SLOT_POSITION_NOT_FOUND"
  | "TEMPLATE_SHIFT_NOT_FOUND"
  | "INVALID_TOKEN"
  | "TOKEN_EXPIRED"
  | "TOKEN_NOT_FOUND"
  | "TOO_MANY_REQUESTS"
  | "TOKEN_USED"
  | "UNAUTHORIZED"
  | "USER_DISABLED"
  | "USER_NOT_PENDING"
  | "USER_PENDING"
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
