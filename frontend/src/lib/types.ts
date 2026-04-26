export type UserStatus = "pending" | "active" | "disabled"

export type User = {
  id: number
  email: string
  name: string
  is_admin: boolean
  status: UserStatus
  version: number
}

export type SetupTokenPurpose = "invitation" | "password_reset"

export type SetupTokenPreview = {
  email: string
  name: string
  purpose: SetupTokenPurpose
}

export type Pagination = {
  page: number
  page_size: number
  total: number
  total_pages: number
}

export type Position = {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
}

export type QualifiedShift = {
  slot_id: number
  position_id: number
  weekday: number
  start_time: string
  end_time: string
  required_headcount: number
}

export type SlotPositionRef = {
  slot_id: number
  position_id: number
}

export type TemplateSlotPosition = {
  id: number
  slot_id: number
  position_id: number
  required_headcount: number
  created_at: string
  updated_at: string
}

export type TemplateSlot = {
  id: number
  template_id: number
  weekday: number
  start_time: string
  end_time: string
  created_at: string
  updated_at: string
  positions: TemplateSlotPosition[]
}

export type Template = {
  id: number
  name: string
  description: string
  is_locked: boolean
  shift_count: number
  created_at: string
  updated_at: string
}

export type TemplateDetail = Template & {
  slots: TemplateSlot[]
}

export type PublicationState =
  | "DRAFT"
  | "COLLECTING"
  | "ASSIGNING"
  | "PUBLISHED"
  | "ACTIVE"
  | "ENDED"

export type Publication = {
  id: number
  template_id: number
  template_name: string
  name: string
  description: string
  state: PublicationState
  submission_start_at: string
  submission_end_at: string
  planned_active_from: string
  planned_active_until: string
  activated_at: string | null
  created_at: string
  updated_at: string
}

export type PublicationSlot = {
  id: number
  weekday: number
  start_time: string
  end_time: string
}

export type PublicationPosition = {
  id: number
  name: string
}

export type AssignmentBoardCandidate = {
  user_id: number
  name: string
  email: string
}

export type AssignmentBoardAssignment = {
  assignment_id: number
  user_id: number
  name: string
  email: string
}

export type AssignmentBoardPosition = {
  position: PublicationPosition
  required_headcount: number
  candidates: AssignmentBoardCandidate[]
  non_candidate_qualified: AssignmentBoardCandidate[]
  assignments: AssignmentBoardAssignment[]
}

export type AssignmentBoardSlot = {
  slot: PublicationSlot
  positions: AssignmentBoardPosition[]
}

export type AssignmentBoard = {
  publication: Publication
  slots: AssignmentBoardSlot[]
}

export type RosterAssignment = {
  assignment_id: number
  user_id: number
  name: string
}

export type RosterPosition = {
  position: PublicationPosition
  required_headcount: number
  assignments: RosterAssignment[]
}

export type RosterSlot = {
  slot: PublicationSlot
  occurrence_date: string
  positions: RosterPosition[]
}

export type RosterWeekday = {
  weekday: number
  slots: RosterSlot[]
}

export type Roster = {
  publication: Publication | null
  week_start: string
  weekdays: RosterWeekday[]
}

export type ShiftChangeType = "swap" | "give_direct" | "give_pool"

export type ShiftChangeState =
  | "pending"
  | "approved"
  | "rejected"
  | "cancelled"
  | "expired"
  | "invalidated"

export type ShiftChangeRequest = {
  id: number
  publication_id: number
  type: ShiftChangeType
  requester_user_id: number
  requester_assignment_id: number
  occurrence_date: string
  counterpart_user_id: number | null
  counterpart_assignment_id: number | null
  counterpart_occurrence_date: string | null
  state: ShiftChangeState
  leave_id: number | null
  decided_by_user_id: number | null
  created_at: string
  decided_at: string | null
  expires_at: string
}

export type PublicationMember = {
  user_id: number
  name: string
}

export type LeaveCategory = "sick" | "personal" | "bereavement"

export type LeaveState = "pending" | "completed" | "failed" | "cancelled"

export type Leave = {
  id: number
  user_id: number
  publication_id: number
  shift_change_request_id: number
  category: LeaveCategory
  reason: string
  state: LeaveState
  share_url: string
  created_at: string
  updated_at: string
  request: ShiftChangeRequest
}

export type LeavePreviewOccurrence = {
  assignment_id: number
  occurrence_date: string
  slot: PublicationSlot
  position: PublicationPosition
  occurrence_start: string
  occurrence_end: string
}
