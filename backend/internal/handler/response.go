package handler

import (
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type userResponse struct {
	ID                 int64                     `json:"id"`
	Email              string                    `json:"email"`
	Name               string                    `json:"name"`
	IsAdmin            bool                      `json:"is_admin"`
	Status             model.UserStatus          `json:"status"`
	Version            int                       `json:"version"`
	LanguagePreference *model.LanguagePreference `json:"language_preference"`
	ThemePreference    *model.ThemePreference    `json:"theme_preference"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type positionResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type templateListResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsLocked    bool      `json:"is_locked"`
	ShiftCount  int       `json:"shift_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type templateResponse struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	IsLocked    bool                   `json:"is_locked"`
	ShiftCount  int                    `json:"shift_count"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Slots       []templateSlotResponse `json:"slots"`
}

type templateSlotResponse struct {
	ID         int64                          `json:"id"`
	TemplateID int64                          `json:"template_id"`
	Weekdays   []int                          `json:"weekdays"`
	StartTime  string                         `json:"start_time"`
	EndTime    string                         `json:"end_time"`
	CreatedAt  time.Time                      `json:"created_at"`
	UpdatedAt  time.Time                      `json:"updated_at"`
	Positions  []templateSlotPositionResponse `json:"positions"`
}

type templateSlotPositionResponse struct {
	ID                int64     `json:"id"`
	SlotID            int64     `json:"slot_id"`
	PositionID        int64     `json:"position_id"`
	RequiredHeadcount int       `json:"required_headcount"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type qualifiedShiftResponse struct {
	SlotID      int64                               `json:"slot_id"`
	Weekday     int                                 `json:"weekday"`
	StartTime   string                              `json:"start_time"`
	EndTime     string                              `json:"end_time"`
	Composition []qualifiedShiftCompositionResponse `json:"composition"`
}

type qualifiedShiftCompositionResponse struct {
	PositionID        int64  `json:"position_id"`
	PositionName      string `json:"position_name"`
	RequiredHeadcount int    `json:"required_headcount"`
}

type publicationResponse struct {
	ID                 int64                  `json:"id"`
	TemplateID         int64                  `json:"template_id"`
	TemplateName       string                 `json:"template_name"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	State              model.PublicationState `json:"state"`
	SubmissionStartAt  time.Time              `json:"submission_start_at"`
	SubmissionEndAt    time.Time              `json:"submission_end_at"`
	PlannedActiveFrom  time.Time              `json:"planned_active_from"`
	PlannedActiveUntil time.Time              `json:"planned_active_until"`
	ActivatedAt        *time.Time             `json:"activated_at"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type publicationSlotResponse struct {
	ID        int64  `json:"id"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type publicationPositionResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type assignmentResponse struct {
	AssignmentID int64  `json:"assignment_id"`
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
}

type assignmentBoardEmployeeResponse struct {
	UserID         int64                         `json:"user_id"`
	Name           string                        `json:"name"`
	Email          string                        `json:"email"`
	PositionIDs    []int64                       `json:"position_ids"`
	SubmittedSlots []assignmentBoardSlotRefValue `json:"submitted_slots"`
}

type assignmentBoardSlotRefValue struct {
	SlotID  int64 `json:"slot_id"`
	Weekday int   `json:"weekday"`
}

type assignmentBoardPositionResponse struct {
	Position          publicationPositionResponse `json:"position"`
	RequiredHeadcount int                         `json:"required_headcount"`
	Assignments       []assignmentResponse        `json:"assignments"`
}

type assignmentBoardSlotResponse struct {
	Slot      publicationSlotResponse           `json:"slot"`
	Positions []assignmentBoardPositionResponse `json:"positions"`
}

type assignmentBoardResponse struct {
	Publication *publicationResponse              `json:"publication"`
	Slots       []assignmentBoardSlotResponse     `json:"slots"`
	Employees   []assignmentBoardEmployeeResponse `json:"employees"`
}

type rosterAssignmentResponse struct {
	AssignmentID int64  `json:"assignment_id"`
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
}

type rosterPositionResponse struct {
	Position          publicationPositionResponse `json:"position"`
	RequiredHeadcount int                         `json:"required_headcount"`
	Assignments       []rosterAssignmentResponse  `json:"assignments"`
}

type rosterSlotResponse struct {
	Slot           publicationSlotResponse  `json:"slot"`
	OccurrenceDate string                   `json:"occurrence_date"`
	Positions      []rosterPositionResponse `json:"positions"`
}

type rosterWeekdayResponse struct {
	Weekday int                  `json:"weekday"`
	Slots   []rosterSlotResponse `json:"slots"`
}

type rosterResponse struct {
	Publication *publicationResponse    `json:"publication"`
	WeekStart   string                  `json:"week_start"`
	Weekdays    []rosterWeekdayResponse `json:"weekdays"`
}

func newUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.Name,
		IsAdmin:            user.IsAdmin,
		Status:             user.Status,
		Version:            user.Version,
		LanguagePreference: user.LanguagePreference,
		ThemePreference:    user.ThemePreference,
	}
}

func newPositionResponse(position *model.Position) positionResponse {
	return positionResponse{
		ID:          position.ID,
		Name:        position.Name,
		Description: position.Description,
		CreatedAt:   position.CreatedAt,
		UpdatedAt:   position.UpdatedAt,
	}
}

func newTemplateResponse(template *model.Template) templateResponse {
	response := templateResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		IsLocked:    template.IsLocked,
		ShiftCount:  template.ShiftCount,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
		Slots:       make([]templateSlotResponse, 0, len(template.Slots)),
	}

	for _, slot := range template.Slots {
		response.Slots = append(response.Slots, newTemplateSlotResponse(slot))
	}

	return response
}

func newTemplateListResponse(template *model.Template) templateListResponse {
	return templateListResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		IsLocked:    template.IsLocked,
		ShiftCount:  template.ShiftCount,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
	}
}

func newQualifiedShiftResponse(shift *model.QualifiedShift) qualifiedShiftResponse {
	response := qualifiedShiftResponse{
		SlotID:      shift.SlotID,
		Weekday:     shift.Weekday,
		StartTime:   shift.StartTime,
		EndTime:     shift.EndTime,
		Composition: make([]qualifiedShiftCompositionResponse, 0, len(shift.Composition)),
	}

	for _, entry := range shift.Composition {
		response.Composition = append(response.Composition, qualifiedShiftCompositionResponse{
			PositionID:        entry.PositionID,
			PositionName:      entry.PositionName,
			RequiredHeadcount: entry.RequiredHeadcount,
		})
	}

	return response
}

func newTemplateSlotResponse(slot *model.TemplateSlot) templateSlotResponse {
	response := templateSlotResponse{
		ID:         slot.ID,
		TemplateID: slot.TemplateID,
		Weekdays:   append([]int(nil), slot.Weekdays...),
		StartTime:  slot.StartTime,
		EndTime:    slot.EndTime,
		CreatedAt:  slot.CreatedAt,
		UpdatedAt:  slot.UpdatedAt,
		Positions:  make([]templateSlotPositionResponse, 0, len(slot.Positions)),
	}

	for _, position := range slot.Positions {
		response.Positions = append(response.Positions, newTemplateSlotPositionResponse(position))
	}

	return response
}

func newTemplateSlotPositionResponse(slotPosition *model.TemplateSlotPosition) templateSlotPositionResponse {
	return templateSlotPositionResponse{
		ID:                slotPosition.ID,
		SlotID:            slotPosition.SlotID,
		PositionID:        slotPosition.PositionID,
		RequiredHeadcount: slotPosition.RequiredHeadcount,
		CreatedAt:         slotPosition.CreatedAt,
		UpdatedAt:         slotPosition.UpdatedAt,
	}
}

func newPublicationResponse(publication *model.Publication) *publicationResponse {
	if publication == nil {
		return nil
	}

	return &publicationResponse{
		ID:                 publication.ID,
		TemplateID:         publication.TemplateID,
		TemplateName:       publication.TemplateName,
		Name:               publication.Name,
		Description:        publication.Description,
		State:              publication.State,
		SubmissionStartAt:  publication.SubmissionStartAt,
		SubmissionEndAt:    publication.SubmissionEndAt,
		PlannedActiveFrom:  publication.PlannedActiveFrom,
		PlannedActiveUntil: publication.PlannedActiveUntil,
		ActivatedAt:        publication.ActivatedAt,
		CreatedAt:          publication.CreatedAt,
		UpdatedAt:          publication.UpdatedAt,
	}
}

func newPublicationSlotResponse(slot *model.TemplateSlot) publicationSlotResponse {
	if slot == nil {
		return publicationSlotResponse{}
	}

	return publicationSlotResponse{
		ID:        slot.ID,
		Weekday:   publicationSlotWeekday(slot),
		StartTime: slot.StartTime,
		EndTime:   slot.EndTime,
	}
}

func publicationSlotWeekday(slot *model.TemplateSlot) int {
	if slot == nil || len(slot.Weekdays) == 0 {
		return 0
	}
	return slot.Weekdays[0]
}

func newPublicationPositionResponse(position *model.Position) publicationPositionResponse {
	if position == nil {
		return publicationPositionResponse{}
	}

	return publicationPositionResponse{
		ID:   position.ID,
		Name: position.Name,
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}
