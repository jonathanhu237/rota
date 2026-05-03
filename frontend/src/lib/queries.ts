import { keepPreviousData, queryOptions } from "@tanstack/react-query"
import { isAxiosError } from "axios"

import api from "./axios"
import type {
  AssignmentBoard,
  Branding,
  Leave,
  LeaveCategory,
  LeavePreviewOccurrence,
  LanguagePreference,
  Pagination,
  Position,
  Publication,
  PublicationMember,
  QualifiedShift,
  ShiftChangeRequest,
  ShiftChangeType,
  SlotRef,
  Template,
  TemplateDetail,
  TemplateSlot,
  TemplateSlotPosition,
  ThemePreference,
  Roster,
  SetupTokenPreview,
  User,
  UserStatus,
} from "./types"

export type UsersResponse = {
  users: User[]
  pagination: Pagination
}

export type UserResponse = {
  user: User
}

export type PasswordResetRequestResponse = {
  message: string
}

export type SetupTokenPreviewResponse = SetupTokenPreview

export type BrandingResponse = Branding

export type PositionsResponse = {
  positions: Position[]
  pagination: Pagination
}

export type PositionResponse = {
  position: Position
}

export type UserPositionsResponse = {
  positions: Position[]
}

export type TemplatesResponse = {
  templates: Template[]
  pagination: Pagination
}

export type TemplateResponse = {
  template: TemplateDetail
}

export type TemplateSlotResponse = {
  slot: TemplateSlot
}

export type TemplateSlotPositionResponse = {
  position: TemplateSlotPosition
}

export type PublicationsResponse = {
  publications: Publication[]
  pagination: Pagination
}

export type PublicationResponse = {
  publication: Publication | null
}

export type PublicationShiftsResponse = {
  shifts: QualifiedShift[]
}

export type MyPublicationSubmissionsResponse = {
  submissions: SlotRef[]
}

export type AssignmentBoardResponse = AssignmentBoard

export type RosterResponse = Roster

export type CreateUserInput = {
  email: string
  name: string
  is_admin: boolean
}

export type UpdateUserInput = {
  email: string
  name: string
  is_admin: boolean
  version: number
}

export type UpdateUserStatusInput = {
  status: UserStatus
  version: number
}

export type SetupPasswordInput = {
  token: string
  password: string
}

export type ConfirmEmailChangeInput = {
  token: string
}

export type ChangeOwnPasswordInput = {
  current_password: string
  new_password: string
}

export type RequestEmailChangeInput = {
  new_email: string
  current_password: string
}

export type UpdateOwnProfileInput = {
  name?: string
  language_preference?: LanguagePreference | null
  theme_preference?: ThemePreference | null
}

export type UpdateBrandingInput = {
  product_name: string
  organization_name: string
  version: number
}

export type CreatePositionInput = {
  name: string
  description: string
}

export type UpdatePositionInput = {
  name: string
  description: string
}

export type ReplaceUserPositionsInput = {
  position_ids: number[]
}

export type CreateTemplateInput = {
  name: string
  description: string
}

export type UpdateTemplateInput = {
  name: string
  description: string
}

export type CreateTemplateSlotInput = {
  weekdays: number[]
  start_time: string
  end_time: string
}

export type UpdateTemplateSlotInput = CreateTemplateSlotInput

export type CreateTemplateSlotPositionInput = {
  position_id: number
  required_headcount: number
}

export type UpdateTemplateSlotPositionInput = CreateTemplateSlotPositionInput

export type CreatePublicationInput = {
  template_id: number
  name: string
  description?: string
  submission_start_at: string
  submission_end_at: string
  planned_active_from: string
  planned_active_until: string
}

export type UpdatePublicationInput = {
  name?: string
  description?: string
  planned_active_until?: string
}

export type CreateAssignmentInput = {
  user_id: number
  slot_id: number
  weekday: number
  position_id: number
}

export const brandingFallback: Branding = {
  product_name: "Rota",
  organization_name: "",
  version: 1,
  created_at: "",
  updated_at: "",
}

export const brandingQueryOptions = queryOptions({
  queryKey: ["branding"],
  queryFn: async () => getBranding(),
})

export const currentUserQueryOptions = queryOptions({
  queryKey: ["auth", "me"],
  queryFn: async () => {
    const res = await api.get<{ user: User }>("/auth/me")
    return res.data.user
  },
  retry: (failureCount, error) => {
    if (isAxiosError(error) && error.response?.status === 401) {
      return false
    }

    return failureCount < 2
  },
})

export async function getBranding() {
  try {
    const res = await api.get<BrandingResponse>("/branding")
    return res.data
  } catch {
    return brandingFallback
  }
}

export const usersQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["users", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<UsersResponse>("/users", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const userQueryOptions = (userID: number) =>
  queryOptions({
    queryKey: ["users", "detail", userID],
    queryFn: async () => {
      const res = await api.get<UserResponse>(`/users/${userID}`)
      return res.data.user
    },
    enabled: userID > 0,
  })

export const positionsQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["positions", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<PositionsResponse>("/positions", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const allPositionsQueryOptions = () =>
  queryOptions({
    queryKey: ["positions", "all"],
    queryFn: async () => {
      const res = await api.get<PositionsResponse>("/positions", {
        params: {
          page: 1,
          page_size: 100,
        },
      })
      return res.data.positions
    },
  })

export const positionQueryOptions = (positionID: number) =>
  queryOptions({
    queryKey: ["positions", "detail", positionID],
    queryFn: async () => {
      const res = await api.get<PositionResponse>(`/positions/${positionID}`)
      return res.data.position
    },
    enabled: positionID > 0,
  })

export const templatesQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["templates", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<TemplatesResponse>("/templates", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const templateQueryOptions = (templateID: number) =>
  queryOptions({
    queryKey: ["templates", "detail", templateID],
    queryFn: async () => {
      const res = await api.get<TemplateResponse>(`/templates/${templateID}`)
      return res.data.template
    },
    enabled: templateID > 0,
  })

export const allTemplatesQueryOptions = () =>
  queryOptions({
    queryKey: ["templates", "all"],
    queryFn: async () => {
      const res = await api.get<TemplatesResponse>("/templates", {
        params: {
          page: 1,
          page_size: 100,
        },
      })
      return res.data.templates
    },
  })

export const userPositionsQueryOptions = (userID: number) =>
  queryOptions({
    queryKey: ["users", "positions", userID],
    queryFn: async () => {
      const res = await api.get<UserPositionsResponse>(`/users/${userID}/positions`)
      return res.data.positions
    },
    enabled: userID > 0,
  })

export const publicationsQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["publications", "list", page, pageSize],
    queryFn: async () => {
      const res = await api.get<PublicationsResponse>("/publications", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const publicationQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", "detail", publicationID],
    queryFn: async () => {
      const res = await api.get<PublicationResponse>(`/publications/${publicationID}`)
      return res.data.publication
    },
    enabled: publicationID > 0,
  })

export const currentPublicationQueryOptions = queryOptions({
  queryKey: ["publications", "current"],
  queryFn: async () => {
    const res = await api.get<PublicationResponse>("/publications/current")
    return res.data.publication
  },
})

export const publicationShiftsQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", "current", "shifts", publicationID],
    queryFn: async () => {
      const res = await api.get<PublicationShiftsResponse>(
        `/publications/${publicationID}/shifts/me`,
      )
      return res.data.shifts
    },
    enabled: publicationID > 0,
  })

export const publicationAssignmentBoardQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", "detail", publicationID, "board"],
    queryFn: async () => {
      const res = await api.get<AssignmentBoardResponse>(
        `/publications/${publicationID}/assignment-board`,
      )
      return res.data
    },
    enabled: publicationID > 0,
  })

export const rosterCurrentQueryOptions = queryOptions({
  queryKey: ["roster", "current"],
  queryFn: async () => {
    const res = await api.get<RosterResponse>("/roster/current")
    return res.data
  },
})

export const publicationRosterQueryOptions = (
  publicationID: number,
  week?: string,
) =>
  queryOptions({
    queryKey: ["publications", "detail", publicationID, "roster", week ?? ""],
    queryFn: async () => {
      const res = await api.get<RosterResponse>(
        `/publications/${publicationID}/roster`,
        { params: week ? { week } : undefined },
      )
      return res.data
    },
    enabled: publicationID > 0,
  })

export const myPublicationSubmissionsQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", "current", "submissions", publicationID],
    queryFn: async () => {
      const res = await api.get<MyPublicationSubmissionsResponse>(
        `/publications/${publicationID}/submissions/me`,
      )
      return res.data.submissions
    },
    enabled: publicationID > 0,
  })

export async function createUser(input: CreateUserInput) {
  const res = await api.post<UserResponse>("/users", input)
  return res.data.user
}

export async function updateUser(userID: number, input: UpdateUserInput) {
  const res = await api.put<UserResponse>(`/users/${userID}`, input)
  return res.data.user
}

export async function updateUserStatus(
  userID: number,
  input: UpdateUserStatusInput,
) {
  const res = await api.patch<UserResponse>(`/users/${userID}/status`, input)
  return res.data.user
}

export async function replaceUserPositions(
  userID: number,
  positionIDs: number[],
) {
  await api.put<undefined, unknown, ReplaceUserPositionsInput>(
    `/users/${userID}/positions`,
    {
      position_ids: positionIDs,
    },
  )
}

export async function requestPasswordReset(email: string) {
  await api.post<PasswordResetRequestResponse>(
    "/auth/password-reset-request",
    { email },
  )
}

export async function previewSetupToken(token: string) {
  const res = await api.get<SetupTokenPreviewResponse>("/auth/setup-token", {
    params: { token },
  })
  return res.data
}

export async function setupPassword(input: SetupPasswordInput) {
  await api.post("/auth/setup-password", input)
}

export async function confirmEmailChange(input: ConfirmEmailChangeInput) {
  await api.post("/auth/confirm-email-change", input)
}

export async function changeOwnPassword(input: ChangeOwnPasswordInput) {
  await api.post("/auth/change-password", input)
}

export async function requestEmailChange(input: RequestEmailChangeInput) {
  await api.post("/users/me/email-change-request", input)
}

export async function updateOwnProfile(input: UpdateOwnProfileInput) {
  const res = await api.put<UserResponse>("/users/me", input)
  return res.data.user
}

export async function updateBranding(input: UpdateBrandingInput) {
  const res = await api.put<BrandingResponse>("/branding", input)
  return res.data
}

export async function resendInvitation(userID: number) {
  await api.post(`/users/${userID}/resend-invitation`)
}

export async function createPosition(input: CreatePositionInput) {
  const res = await api.post<PositionResponse>("/positions", input)
  return res.data.position
}

export async function updatePosition(
  positionID: number,
  input: UpdatePositionInput,
) {
  const res = await api.put<PositionResponse>(`/positions/${positionID}`, input)
  return res.data.position
}

export async function deletePosition(positionID: number) {
  await api.delete(`/positions/${positionID}`)
}

export async function createTemplate(input: CreateTemplateInput) {
  const res = await api.post<TemplateResponse>("/templates", input)
  return res.data.template
}

export async function updateTemplate(
  templateID: number,
  input: UpdateTemplateInput,
) {
  const res = await api.put<TemplateResponse>(`/templates/${templateID}`, input)
  return res.data.template
}

export async function deleteTemplate(templateID: number) {
  await api.delete(`/templates/${templateID}`)
}

export async function cloneTemplate(templateID: number) {
  const res = await api.post<TemplateResponse>(`/templates/${templateID}/clone`)
  return res.data.template
}

export async function createTemplateSlot(
  templateID: number,
  input: CreateTemplateSlotInput,
) {
  const res = await api.post<TemplateSlotResponse>(
    `/templates/${templateID}/slots`,
    input,
  )
  return res.data.slot
}

export async function updateTemplateSlot(
  templateID: number,
  slotID: number,
  input: UpdateTemplateSlotInput,
) {
  const res = await api.patch<TemplateSlotResponse>(
    `/templates/${templateID}/slots/${slotID}`,
    input,
  )
  return res.data.slot
}

export async function deleteTemplateSlot(templateID: number, slotID: number) {
  await api.delete(`/templates/${templateID}/slots/${slotID}`)
}

export async function createTemplateSlotPosition(
  templateID: number,
  slotID: number,
  input: CreateTemplateSlotPositionInput,
) {
  const res = await api.post<TemplateSlotPositionResponse>(
    `/templates/${templateID}/slots/${slotID}/positions`,
    input,
  )
  return res.data.position
}

export async function updateTemplateSlotPosition(
  templateID: number,
  slotID: number,
  positionEntryID: number,
  input: UpdateTemplateSlotPositionInput,
) {
  const res = await api.patch<TemplateSlotPositionResponse>(
    `/templates/${templateID}/slots/${slotID}/positions/${positionEntryID}`,
    input,
  )
  return res.data.position
}

export async function deleteTemplateSlotPosition(
  templateID: number,
  slotID: number,
  positionEntryID: number,
) {
  await api.delete(
    `/templates/${templateID}/slots/${slotID}/positions/${positionEntryID}`,
  )
}

export async function createPublication(input: CreatePublicationInput) {
  const res = await api.post<PublicationResponse>("/publications", {
    ...input,
    submission_start_at: toApiTimestamp(input.submission_start_at),
    submission_end_at: toApiTimestamp(input.submission_end_at),
    planned_active_from: toApiTimestamp(input.planned_active_from),
    planned_active_until: toApiTimestamp(input.planned_active_until),
  })
  return res.data.publication
}

export async function updatePublication(
  publicationID: number,
  input: UpdatePublicationInput,
) {
  const res = await api.patch<PublicationResponse>(
    `/publications/${publicationID}`,
    input,
  )
  return res.data.publication
}

export async function activatePublication(publicationID: number) {
  await api.post(`/publications/${publicationID}/activate`)
}

export async function publishPublication(publicationID: number) {
  await api.post(`/publications/${publicationID}/publish`)
}

export async function endPublication(publicationID: number) {
  await api.post(`/publications/${publicationID}/end`)
}

export async function deletePublication(publicationID: number) {
  await api.delete(`/publications/${publicationID}`)
}

function toApiTimestamp(value: string) {
  return new Date(value).toISOString()
}

export async function createAssignment(
  publicationID: number,
  input: CreateAssignmentInput,
) {
  await api.post(`/publications/${publicationID}/assignments`, input)
}

export async function autoAssignPublication(publicationID: number) {
  const res = await api.post<AssignmentBoardResponse>(
    `/publications/${publicationID}/auto-assign`,
  )
  return res.data
}

export async function deleteAssignment(
  publicationID: number,
  assignmentID: number,
) {
  await api.delete(`/publications/${publicationID}/assignments/${assignmentID}`)
}

export async function createAvailabilitySubmission(
  publicationID: number,
  slotID: number,
  weekday: number,
) {
  await api.post(`/publications/${publicationID}/submissions`, {
    slot_id: slotID,
    weekday,
  })
}

export async function deleteAvailabilitySubmission(
  publicationID: number,
  slotID: number,
  weekday: number,
) {
  await api.delete(
    `/publications/${publicationID}/submissions/${slotID}/${weekday}`,
  )
}

export type ShiftChangeRequestResponse = {
  request: ShiftChangeRequest
}

export type ShiftChangeRequestListResponse = {
  requests: ShiftChangeRequest[]
}

export type PublicationMembersResponse = {
  members: PublicationMember[]
}

export type UnreadCountResponse = {
  count: number
}

export type LeaveResponse = {
  leave: Leave
}

export type LeaveListResponse = {
  leaves: Leave[]
}

export type LeavePreviewResponse = {
  occurrences: LeavePreviewOccurrence[]
}

export type CreateShiftChangeInput = {
  type: ShiftChangeType
  requester_assignment_id: number
  occurrence_date: string
  counterpart_user_id?: number | null
  counterpart_assignment_id?: number | null
  counterpart_occurrence_date?: string | null
}

export type CreateLeaveInput = {
  assignment_id: number
  occurrence_date: string
  type: Exclude<ShiftChangeType, "swap">
  counterpart_user_id?: number | null
  category: LeaveCategory
  reason?: string
}

export const shiftChangeRequestsQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", publicationID, "shift-changes"] as const,
    queryFn: async () => {
      const res = await api.get<ShiftChangeRequestListResponse>(
        `/publications/${publicationID}/shift-changes`,
      )
      return res.data.requests
    },
    enabled: publicationID > 0,
  })

export const publicationMembersQueryOptions = (publicationID: number) =>
  queryOptions({
    queryKey: ["publications", publicationID, "members"] as const,
    queryFn: async () => {
      const res = await api.get<PublicationMembersResponse>(
        `/publications/${publicationID}/members`,
      )
      return res.data.members
    },
    enabled: publicationID > 0,
  })

export const unreadNotificationsQueryOptions = queryOptions({
  queryKey: ["me", "notifications", "unread-count"] as const,
  queryFn: async () => {
    const res = await api.get<UnreadCountResponse>(
      "/users/me/notifications/unread-count",
    )
    return res.data.count
  },
})

export const myLeavesQueryOptions = (page: number, pageSize: number) =>
  queryOptions({
    queryKey: ["me", "leaves", page, pageSize] as const,
    queryFn: async () => {
      const res = await api.get<LeaveListResponse>("/users/me/leaves", {
        params: {
          page,
          page_size: pageSize,
        },
      })
      return res.data.leaves
    },
    placeholderData: keepPreviousData,
  })

export const leavePreviewQueryOptions = (from: string, to: string) =>
  queryOptions({
    queryKey: ["me", "leaves", "preview", from, to] as const,
    queryFn: async () => {
      const res = await api.get<LeavePreviewResponse>(
        "/users/me/leaves/preview",
        {
          params: { from, to },
        },
      )
      return res.data.occurrences
    },
    enabled: from !== "" && to !== "",
  })

export const leaveQueryOptions = (leaveID: number) =>
  queryOptions({
    queryKey: ["leaves", leaveID] as const,
    queryFn: async () => {
      const res = await api.get<LeaveResponse>(`/leaves/${leaveID}`)
      return res.data.leave
    },
    enabled: leaveID > 0,
  })

export const publicationLeavesQueryOptions = (
  publicationID: number,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: ["publications", publicationID, "leaves", page, pageSize] as const,
    queryFn: async () => {
      const res = await api.get<LeaveListResponse>(
        `/publications/${publicationID}/leaves`,
        {
          params: { page, page_size: pageSize },
        },
      )
      return res.data.leaves
    },
    enabled: publicationID > 0,
    placeholderData: keepPreviousData,
  })

export async function createShiftChangeRequest(
  publicationID: number,
  input: CreateShiftChangeInput,
) {
  const res = await api.post<ShiftChangeRequestResponse>(
    `/publications/${publicationID}/shift-changes`,
    input,
  )
  return res.data.request
}

export async function approveShiftChangeRequest(
  publicationID: number,
  requestID: number,
) {
  await api.post(
    `/publications/${publicationID}/shift-changes/${requestID}/approve`,
  )
}

export async function rejectShiftChangeRequest(
  publicationID: number,
  requestID: number,
) {
  await api.post(
    `/publications/${publicationID}/shift-changes/${requestID}/reject`,
  )
}

export async function cancelShiftChangeRequest(
  publicationID: number,
  requestID: number,
) {
  await api.post(
    `/publications/${publicationID}/shift-changes/${requestID}/cancel`,
  )
}

export async function createLeave(input: CreateLeaveInput) {
  const res = await api.post<LeaveResponse>("/leaves", input)
  return res.data.leave
}

export async function cancelLeave(leaveID: number) {
  await api.post(`/leaves/${leaveID}/cancel`)
}
