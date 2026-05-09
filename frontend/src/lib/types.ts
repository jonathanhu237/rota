export type UserStatus = "pending" | "active" | "disabled"
export type LanguagePreference = "zh" | "en"
export type ThemePreference = "light" | "dark" | "system"

export type User = {
  id: number
  email: string
  name: string
  is_admin: boolean
  status: UserStatus
  version: number
  language_preference: LanguagePreference | null
  theme_preference: ThemePreference | null
}

export type Branding = {
  product_name: string
  organization_name: string
  version: number
  created_at: string
  updated_at: string
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
  weekday: number
  start_time: string
  end_time: string
  composition: QualifiedShiftComposition[]
}

export type QualifiedShiftComposition = {
  position_id: number
  position_name: string
  required_headcount: number
}

export type SlotRef = {
  slot_id: number
  weekday: number
}

export type TemplateSlotPosition = {
  id: number
  slot_id: number
  position_id: number
  required_headcount: number
  attendance_responsible: boolean
  created_at: string
  updated_at: string
}

export type TemplateSlot = {
  id: number
  template_id: number
  weekdays: number[]
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
  overtime_entry_window_hours?: number
  activated_at: string | null
  created_at: string
  updated_at: string
}

export type AttendanceStatus = "pending" | "present" | "late" | "absent"

export type AttendanceArrivalRecord = {
  id: number
  publication_id: number
  assignment_id: number
  occurrence_date: string
  user_id: number
  user_name: string
  user_email: string
  arrived_at: string
  recorded_by_user_id: number | null
  recorded_at: string
  updated_by_user_id: number | null
  updated_at: string
  status?: AttendanceStatus
}

export type AttendanceRosterEntry = {
  assignment_id: number
  position_id: number
  position_name: string
  attendance_responsible: boolean
  user_id: number
  user_name: string
  user_email: string
  status: AttendanceStatus
  record: AttendanceArrivalRecord | null
}

export type AttendanceOvertimeRecord = {
  id: number
  publication_id: number
  slot_id: number
  weekday: number
  occurrence_date: string
  user_id: number
  user_name: string
  user_email: string
  hours: number
  note: string
  recorded_by_user_id: number | null
  recorded_at: string
  updated_by_user_id: number | null
  updated_at: string
}

export type AttendanceShift = {
  publication_id: number
  slot_id: number
  weekday: number
  start_time: string
  end_time: string
  occurrence_date: string
  scheduled_start: string
  scheduled_end: string
  arrival_window_open: boolean
  overtime_window_open: boolean
  roster: AttendanceRosterEntry[]
  orphan_arrivals: AttendanceArrivalRecord[]
  overtime_records: AttendanceOvertimeRecord[]
}

export type AttendanceShiftSummary = {
  slot_id: number
  weekday: number
  occurrence_date: string
  scheduled_start: string
  scheduled_end: string
  roster_count: number
  pending_count: number
  present_count: number
  late_count: number
  absent_count: number
  orphan_count: number
  overtime_count: number
}

export type LeaderAttendance = {
  publication: Publication | null
  shifts: AttendanceShift[]
}

export type AdminAttendanceDay = {
  publication: Publication
  date: string
  shifts: AttendanceShiftSummary[]
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

export type AssignmentBoardEmployee = {
  user_id: number
  name: string
  email: string
  position_ids: number[]
  submitted_slots: SlotRef[]
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
  assignments: AssignmentBoardAssignment[]
}

export type AssignmentBoardSlot = {
  slot: PublicationSlot
  positions: AssignmentBoardPosition[]
}

export type AssignmentBoard = {
  publication: Publication
  slots: AssignmentBoardSlot[]
  employees: AssignmentBoardEmployee[]
}

export type AdminAvailabilityEmployee = {
  user_id: number
  name: string
  email: string
  positions: PublicationPosition[]
  submitted_count: number
}

export type AdminAvailabilityBoard = {
  publication: Publication
  employees: AdminAvailabilityEmployee[]
  pagination: Pagination
}

export type AdminAvailabilitySlotPosition = {
  position: PublicationPosition
  required_headcount: number
}

export type AdminAvailabilitySlot = {
  slot: PublicationSlot
  positions: AdminAvailabilitySlotPosition[]
}

export type AdminAvailabilityCell = {
  slot_id: number
  weekday: number
  eligible: boolean
  submitted: boolean
}

export type AdminAvailabilityDetail = {
  publication: Publication
  user: User
  positions: PublicationPosition[]
  slots: AdminAvailabilitySlot[]
  submissions: SlotRef[]
  cells: AdminAvailabilityCell[]
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

export type LeavePoolState = LeaveState | "all"

export type LeaveActionDisabledReason =
  | ""
  | "not_qualified"
  | "admin_view_only"

export type LeaveActions = {
  can_claim: boolean
  can_approve: boolean
  can_reject: boolean
  can_cancel: boolean
  disabled_reason?: LeaveActionDisabledReason
}

export type LeaveShiftContext = {
  assignment_id: number
  slot_id: number
  weekday: number
  start_time: string
  end_time: string
  position_id: number
  position_name: string
  occurrence_start: string
  occurrence_end: string
}

export type LeaveUrgency = {
  occurrence_start: string
  seconds_until_start: number
  starts_within_24_hours: boolean
}

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
  requester_name?: string
  counterpart_name?: string | null
  substitute_name?: string | null
  shift?: LeaveShiftContext | null
  urgency?: LeaveUrgency | null
  actions?: LeaveActions
}

export type LeavePoolResponse = {
  leaves: Leave[]
  page: number
  page_size: number
  total_count: number
}

export type LeaveDirectCandidate = {
  user_id: number
  name: string
}

export type LeavePreviewOccurrence = {
  assignment_id: number
  occurrence_date: string
  slot: PublicationSlot
  position: PublicationPosition
  occurrence_start: string
  occurrence_end: string
  direct_candidates: LeaveDirectCandidate[]
}
