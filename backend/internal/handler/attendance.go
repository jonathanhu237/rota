package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type attendanceService interface {
	ListCurrentAttendance(ctx context.Context, userID int64) (*service.LeaderAttendanceResult, error)
	RecordLeaderArrival(ctx context.Context, input service.RecordLeaderArrivalInput) (*service.AttendanceShiftDetail, error)
	RecordLeaderOvertime(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	ListAdminAttendance(ctx context.Context, input service.ListAdminAttendanceInput) (*service.AdminAttendanceDayResult, error)
	GetAdminShiftAttendance(ctx context.Context, input service.GetAdminShiftAttendanceInput) (*service.AttendanceShiftDetail, error)
	AdminUpsertArrival(ctx context.Context, input service.AdminUpsertArrivalInput) (*service.AttendanceShiftDetail, error)
	AdminClearArrival(ctx context.Context, input service.AdminClearArrivalInput) error
	AdminCreateOvertime(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	AdminUpdateOvertime(ctx context.Context, input service.AdminUpdateOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	AdminDeleteOvertime(ctx context.Context, input service.AdminClearArrivalInput) error
	UpdateAttendanceSettings(ctx context.Context, input service.UpdateAttendanceSettingsInput) (*model.Publication, error)
}

type AttendanceHandler struct {
	attendanceService attendanceService
}

type leaderArrivalRequest struct {
	PublicationID  int64      `json:"publication_id"`
	SlotID         int64      `json:"slot_id"`
	AssignmentID   int64      `json:"assignment_id"`
	OccurrenceDate string     `json:"occurrence_date"`
	UserID         int64      `json:"user_id"`
	ArrivedAt      *time.Time `json:"arrived_at"`
}

type overtimeRequest struct {
	PublicationID  int64   `json:"publication_id"`
	SlotID         int64   `json:"slot_id"`
	OccurrenceDate string  `json:"occurrence_date"`
	UserID         int64   `json:"user_id"`
	Hours          float64 `json:"hours"`
	Note           string  `json:"note"`
}

type adminArrivalRequest struct {
	SlotID         int64     `json:"slot_id"`
	AssignmentID   int64     `json:"assignment_id"`
	OccurrenceDate string    `json:"occurrence_date"`
	UserID         int64     `json:"user_id"`
	ArrivedAt      time.Time `json:"arrived_at"`
}

type adminUpdateOvertimeRequest struct {
	Hours float64 `json:"hours"`
	Note  string  `json:"note"`
}

type attendanceSettingsRequest struct {
	OvertimeEntryWindowHours float64 `json:"overtime_entry_window_hours"`
}

type currentAttendanceResponse struct {
	Publication *publicationResponse      `json:"publication"`
	Shifts      []attendanceShiftResponse `json:"shifts"`
}

type adminAttendanceDayResponse struct {
	Publication *publicationResponse             `json:"publication"`
	Date        string                           `json:"date"`
	Shifts      []attendanceShiftSummaryResponse `json:"shifts"`
}

type attendanceShiftDetailResponse struct {
	Shift attendanceShiftResponse `json:"shift"`
}

type attendanceOvertimeDetailResponse struct {
	Overtime attendanceOvertimeResponse `json:"overtime"`
}

type attendanceSettingsResponse struct {
	Publication *publicationResponse `json:"publication"`
}

type attendanceShiftSummaryResponse struct {
	SlotID         int64     `json:"slot_id"`
	Weekday        int       `json:"weekday"`
	OccurrenceDate string    `json:"occurrence_date"`
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`
	RosterCount    int       `json:"roster_count"`
	PendingCount   int       `json:"pending_count"`
	PresentCount   int       `json:"present_count"`
	LateCount      int       `json:"late_count"`
	AbsentCount    int       `json:"absent_count"`
	OrphanCount    int       `json:"orphan_count"`
	OvertimeCount  int       `json:"overtime_count"`
}

type attendanceShiftResponse struct {
	PublicationID      int64                        `json:"publication_id"`
	SlotID             int64                        `json:"slot_id"`
	Weekday            int                          `json:"weekday"`
	StartTime          string                       `json:"start_time"`
	EndTime            string                       `json:"end_time"`
	OccurrenceDate     string                       `json:"occurrence_date"`
	ScheduledStart     time.Time                    `json:"scheduled_start"`
	ScheduledEnd       time.Time                    `json:"scheduled_end"`
	ArrivalWindowOpen  bool                         `json:"arrival_window_open"`
	OvertimeWindowOpen bool                         `json:"overtime_window_open"`
	Roster             []attendanceRosterResponse   `json:"roster"`
	OrphanArrivals     []attendanceArrivalResponse  `json:"orphan_arrivals"`
	OvertimeRecords    []attendanceOvertimeResponse `json:"overtime_records"`
}

type attendanceRosterResponse struct {
	AssignmentID          int64                      `json:"assignment_id"`
	PositionID            int64                      `json:"position_id"`
	PositionName          string                     `json:"position_name"`
	AttendanceResponsible bool                       `json:"attendance_responsible"`
	UserID                int64                      `json:"user_id"`
	UserName              string                     `json:"user_name"`
	UserEmail             string                     `json:"user_email"`
	Status                model.AttendanceStatus     `json:"status"`
	Record                *attendanceArrivalResponse `json:"record"`
}

type attendanceArrivalResponse struct {
	ID               int64                   `json:"id"`
	PublicationID    int64                   `json:"publication_id"`
	AssignmentID     int64                   `json:"assignment_id"`
	OccurrenceDate   string                  `json:"occurrence_date"`
	UserID           int64                   `json:"user_id"`
	UserName         string                  `json:"user_name"`
	UserEmail        string                  `json:"user_email"`
	ArrivedAt        time.Time               `json:"arrived_at"`
	RecordedByUserID *int64                  `json:"recorded_by_user_id"`
	RecordedAt       time.Time               `json:"recorded_at"`
	UpdatedByUserID  *int64                  `json:"updated_by_user_id"`
	UpdatedAt        time.Time               `json:"updated_at"`
	Status           *model.AttendanceStatus `json:"status,omitempty"`
}

type attendanceOvertimeResponse struct {
	ID               int64     `json:"id"`
	PublicationID    int64     `json:"publication_id"`
	SlotID           int64     `json:"slot_id"`
	Weekday          int       `json:"weekday"`
	OccurrenceDate   string    `json:"occurrence_date"`
	UserID           int64     `json:"user_id"`
	UserName         string    `json:"user_name"`
	UserEmail        string    `json:"user_email"`
	Hours            float64   `json:"hours"`
	Note             string    `json:"note"`
	RecordedByUserID *int64    `json:"recorded_by_user_id"`
	RecordedAt       time.Time `json:"recorded_at"`
	UpdatedByUserID  *int64    `json:"updated_by_user_id"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func NewAttendanceHandler(attendanceService attendanceService) *AttendanceHandler {
	return &AttendanceHandler{attendanceService: attendanceService}
}

func (h *AttendanceHandler) Current(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	result, err := h.attendanceService.ListCurrentAttendance(r.Context(), user.ID)
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	shifts := make([]attendanceShiftResponse, 0, len(result.Shifts))
	for _, shift := range result.Shifts {
		shifts = append(shifts, newAttendanceShiftResponse(shift))
	}
	writeData(w, http.StatusOK, currentAttendanceResponse{
		Publication: newPublicationResponse(result.Publication),
		Shifts:      shifts,
	})
}

func (h *AttendanceHandler) RecordLeaderArrival(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	var req leaderArrivalRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseDate(req.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}

	shift, err := h.attendanceService.RecordLeaderArrival(r.Context(), service.RecordLeaderArrivalInput{
		ActorUserID:    user.ID,
		PublicationID:  req.PublicationID,
		SlotID:         req.SlotID,
		AssignmentID:   req.AssignmentID,
		OccurrenceDate: occurrenceDate,
		UserID:         req.UserID,
		ArrivedAt:      req.ArrivedAt,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, attendanceShiftDetailResponse{
		Shift: newAttendanceShiftResponse(shift),
	})
}

func (h *AttendanceHandler) RecordLeaderOvertime(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	var req overtimeRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseDate(req.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}

	record, err := h.attendanceService.RecordLeaderOvertime(r.Context(), service.RecordOvertimeInput{
		ActorUserID:    user.ID,
		PublicationID:  req.PublicationID,
		SlotID:         req.SlotID,
		OccurrenceDate: occurrenceDate,
		UserID:         req.UserID,
		Hours:          req.Hours,
		Note:           req.Note,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, attendanceOvertimeDetailResponse{
		Overtime: newAttendanceOvertimeResponse(record),
	})
}

func (h *AttendanceHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid date")
		return
	}

	result, err := h.attendanceService.ListAdminAttendance(r.Context(), service.ListAdminAttendanceInput{
		PublicationID:  publicationID,
		OccurrenceDate: date,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	shifts := make([]attendanceShiftSummaryResponse, 0, len(result.Shifts))
	for _, shift := range result.Shifts {
		shifts = append(shifts, newAttendanceShiftSummaryResponse(shift))
	}
	writeData(w, http.StatusOK, adminAttendanceDayResponse{
		Publication: newPublicationResponse(result.Publication),
		Date:        result.Date.Format("2006-01-02"),
		Shifts:      shifts,
	})
}

func (h *AttendanceHandler) GetAdminShift(w http.ResponseWriter, r *http.Request) {
	publicationID, slotID, occurrenceDate, ok := h.parseShiftPath(w, r)
	if !ok {
		return
	}

	shift, err := h.attendanceService.GetAdminShiftAttendance(r.Context(), service.GetAdminShiftAttendanceInput{
		PublicationID:  publicationID,
		SlotID:         slotID,
		OccurrenceDate: occurrenceDate,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, attendanceShiftDetailResponse{
		Shift: newAttendanceShiftResponse(shift),
	})
}

func (h *AttendanceHandler) AdminUpsertArrival(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	var req adminArrivalRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseDate(req.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}

	shift, err := h.attendanceService.AdminUpsertArrival(r.Context(), service.AdminUpsertArrivalInput{
		ActorUserID:    user.ID,
		PublicationID:  publicationID,
		SlotID:         req.SlotID,
		AssignmentID:   req.AssignmentID,
		OccurrenceDate: occurrenceDate,
		UserID:         req.UserID,
		ArrivedAt:      req.ArrivedAt,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, attendanceShiftDetailResponse{
		Shift: newAttendanceShiftResponse(shift),
	})
}

func (h *AttendanceHandler) AdminClearArrival(w http.ResponseWriter, r *http.Request) {
	user, publicationID, recordID, ok := h.parseRecordPath(w, r)
	if !ok {
		return
	}

	if err := h.attendanceService.AdminClearArrival(r.Context(), service.AdminClearArrivalInput{
		ActorUserID:   user.ID,
		PublicationID: publicationID,
		RecordID:      recordID,
	}); err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AttendanceHandler) AdminCreateOvertime(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	var req overtimeRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseDate(req.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}

	record, err := h.attendanceService.AdminCreateOvertime(r.Context(), service.RecordOvertimeInput{
		ActorUserID:    user.ID,
		PublicationID:  publicationID,
		SlotID:         req.SlotID,
		OccurrenceDate: occurrenceDate,
		UserID:         req.UserID,
		Hours:          req.Hours,
		Note:           req.Note,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, attendanceOvertimeDetailResponse{
		Overtime: newAttendanceOvertimeResponse(record),
	})
}

func (h *AttendanceHandler) AdminUpdateOvertime(w http.ResponseWriter, r *http.Request) {
	user, publicationID, recordID, ok := h.parseRecordPath(w, r)
	if !ok {
		return
	}

	var req adminUpdateOvertimeRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	record, err := h.attendanceService.AdminUpdateOvertime(r.Context(), service.AdminUpdateOvertimeInput{
		ActorUserID:   user.ID,
		PublicationID: publicationID,
		RecordID:      recordID,
		Hours:         req.Hours,
		Note:          req.Note,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, attendanceOvertimeDetailResponse{
		Overtime: newAttendanceOvertimeResponse(record),
	})
}

func (h *AttendanceHandler) AdminDeleteOvertime(w http.ResponseWriter, r *http.Request) {
	user, publicationID, recordID, ok := h.parseRecordPath(w, r)
	if !ok {
		return
	}

	if err := h.attendanceService.AdminDeleteOvertime(r.Context(), service.AdminClearArrivalInput{
		ActorUserID:   user.ID,
		PublicationID: publicationID,
		RecordID:      recordID,
	}); err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AttendanceHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	var req attendanceSettingsRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	publication, err := h.attendanceService.UpdateAttendanceSettings(r.Context(), service.UpdateAttendanceSettingsInput{
		ActorUserID:              user.ID,
		PublicationID:            publicationID,
		OvertimeEntryWindowHours: req.OvertimeEntryWindowHours,
	})
	if err != nil {
		h.writeAttendanceServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, attendanceSettingsResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *AttendanceHandler) parseShiftPath(w http.ResponseWriter, r *http.Request) (int64, int64, time.Time, bool) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return 0, 0, time.Time{}, false
	}
	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid slot id")
		return 0, 0, time.Time{}, false
	}
	occurrenceDate, err := parseDate(r.PathValue("occurrence_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return 0, 0, time.Time{}, false
	}
	return publicationID, slotID, occurrenceDate, true
}

func (h *AttendanceHandler) parseRecordPath(w http.ResponseWriter, r *http.Request) (*model.User, int64, int64, bool) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return nil, 0, 0, false
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return nil, 0, 0, false
	}
	recordID, err := parsePathID(r, "record_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid attendance record id")
		return nil, 0, 0, false
	}
	return user, publicationID, recordID, true
}

func newAttendanceShiftSummaryResponse(summary *service.AttendanceShiftSummary) attendanceShiftSummaryResponse {
	return attendanceShiftSummaryResponse{
		SlotID:         summary.SlotID,
		Weekday:        summary.Weekday,
		OccurrenceDate: summary.OccurrenceDate.Format("2006-01-02"),
		ScheduledStart: summary.ScheduledStart,
		ScheduledEnd:   summary.ScheduledEnd,
		RosterCount:    summary.RosterCount,
		PendingCount:   summary.PendingCount,
		PresentCount:   summary.PresentCount,
		LateCount:      summary.LateCount,
		AbsentCount:    summary.AbsentCount,
		OrphanCount:    summary.OrphanCount,
		OvertimeCount:  summary.OvertimeCount,
	}
}

func newAttendanceShiftResponse(shift *service.AttendanceShiftDetail) attendanceShiftResponse {
	roster := make([]attendanceRosterResponse, 0, len(shift.Roster))
	for _, row := range shift.Roster {
		var record *attendanceArrivalResponse
		if row.Record != nil {
			response := newAttendanceArrivalResponse(row.Record)
			record = &response
		}
		roster = append(roster, attendanceRosterResponse{
			AssignmentID:          row.AssignmentID,
			PositionID:            row.PositionID,
			PositionName:          row.PositionName,
			AttendanceResponsible: row.AttendanceResponsible,
			UserID:                row.UserID,
			UserName:              row.UserName,
			UserEmail:             row.UserEmail,
			Status:                row.Status,
			Record:                record,
		})
	}

	orphanArrivals := make([]attendanceArrivalResponse, 0, len(shift.OrphanArrivals))
	for _, orphan := range shift.OrphanArrivals {
		response := newAttendanceArrivalResponse(orphan.Record)
		status := orphan.Status
		response.Status = &status
		orphanArrivals = append(orphanArrivals, response)
	}

	overtimeRecords := make([]attendanceOvertimeResponse, 0, len(shift.OvertimeRecords))
	for _, record := range shift.OvertimeRecords {
		overtimeRecords = append(overtimeRecords, newAttendanceOvertimeResponse(record))
	}

	publicationID := int64(0)
	if shift.Publication != nil {
		publicationID = shift.Publication.ID
	}
	return attendanceShiftResponse{
		PublicationID:      publicationID,
		SlotID:             shift.SlotID,
		Weekday:            shift.Weekday,
		StartTime:          shift.StartTime,
		EndTime:            shift.EndTime,
		OccurrenceDate:     shift.OccurrenceDate.Format("2006-01-02"),
		ScheduledStart:     shift.ScheduledStart,
		ScheduledEnd:       shift.ScheduledEnd,
		ArrivalWindowOpen:  shift.ArrivalWindowOpen,
		OvertimeWindowOpen: shift.OvertimeWindowOpen,
		Roster:             roster,
		OrphanArrivals:     orphanArrivals,
		OvertimeRecords:    overtimeRecords,
	}
}

func newAttendanceArrivalResponse(record *model.AttendanceRecord) attendanceArrivalResponse {
	return attendanceArrivalResponse{
		ID:               record.ID,
		PublicationID:    record.PublicationID,
		AssignmentID:     record.AssignmentID,
		OccurrenceDate:   record.OccurrenceDate.Format("2006-01-02"),
		UserID:           record.UserID,
		UserName:         record.UserName,
		UserEmail:        record.UserEmail,
		ArrivedAt:        record.ArrivedAt,
		RecordedByUserID: record.RecordedByUserID,
		RecordedAt:       record.RecordedAt,
		UpdatedByUserID:  record.UpdatedByUserID,
		UpdatedAt:        record.UpdatedAt,
	}
}

func newAttendanceOvertimeResponse(record *model.AttendanceOvertimeRecord) attendanceOvertimeResponse {
	return attendanceOvertimeResponse{
		ID:               record.ID,
		PublicationID:    record.PublicationID,
		SlotID:           record.SlotID,
		Weekday:          record.Weekday,
		OccurrenceDate:   record.OccurrenceDate.Format("2006-01-02"),
		UserID:           record.UserID,
		UserName:         record.UserName,
		UserEmail:        record.UserEmail,
		Hours:            record.Hours,
		Note:             record.Note,
		RecordedByUserID: record.RecordedByUserID,
		RecordedAt:       record.RecordedAt,
		UpdatedByUserID:  record.UpdatedByUserID,
		UpdatedAt:        record.UpdatedAt,
	}
}

func (h *AttendanceHandler) writeAttendanceServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrInvalidOccurrenceDate):
		writeError(w, http.StatusBadRequest, "INVALID_OCCURRENCE_DATE", "Invalid occurrence date")
	case errors.Is(err, service.ErrPublicationNotFound):
		writeError(w, http.StatusNotFound, "PUBLICATION_NOT_FOUND", "Publication not found")
	case errors.Is(err, service.ErrTemplateSlotNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SLOT_NOT_FOUND", "Template slot not found")
	case errors.Is(err, service.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, service.ErrAttendanceRecordNotFound):
		writeError(w, http.StatusNotFound, "ATTENDANCE_RECORD_NOT_FOUND", "Attendance record not found")
	case errors.Is(err, service.ErrAttendanceNotLeader):
		writeError(w, http.StatusForbidden, "ATTENDANCE_NOT_LEADER", "Caller is not the attendance leader")
	case errors.Is(err, service.ErrAttendanceWindowClosed):
		writeError(w, http.StatusConflict, "ATTENDANCE_WINDOW_CLOSED", "Attendance window is closed")
	case errors.Is(err, service.ErrAttendanceAlreadyRecorded):
		writeError(w, http.StatusConflict, "ATTENDANCE_ALREADY_RECORDED", "Attendance already recorded")
	case errors.Is(err, service.ErrAttendanceRosterStale):
		writeError(w, http.StatusConflict, "ATTENDANCE_ROSTER_STALE", "Attendance roster is stale")
	case errors.Is(err, service.ErrAttendanceResponsibleRequired):
		writeError(w, http.StatusConflict, "ATTENDANCE_RESPONSIBLE_REQUIRED", "Attendance responsible position required")
	case errors.Is(err, service.ErrPublicationNotActive):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_ACTIVE", "Publication is not active")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func parseDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("missing date")
	}
	return time.Parse("2006-01-02", raw)
}
