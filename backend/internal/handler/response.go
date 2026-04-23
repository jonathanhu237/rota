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
	ID      int64            `json:"id"`
	Email   string           `json:"email"`
	Name    string           `json:"name"`
	IsAdmin bool             `json:"is_admin"`
	Status  model.UserStatus `json:"status"`
	Version int              `json:"version"`
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
	Weekday    int                            `json:"weekday"`
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

type templateShiftResponse struct {
	ID                int64     `json:"id"`
	TemplateID        int64     `json:"template_id"`
	Weekday           int       `json:"weekday"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	PositionID        int64     `json:"position_id"`
	RequiredHeadcount int       `json:"required_headcount"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type publicationResponse struct {
	ID                int64                  `json:"id"`
	TemplateID        int64                  `json:"template_id"`
	TemplateName      string                 `json:"template_name"`
	Name              string                 `json:"name"`
	State             model.PublicationState `json:"state"`
	SubmissionStartAt time.Time              `json:"submission_start_at"`
	SubmissionEndAt   time.Time              `json:"submission_end_at"`
	PlannedActiveFrom time.Time              `json:"planned_active_from"`
	ActivatedAt       *time.Time             `json:"activated_at"`
	EndedAt           *time.Time             `json:"ended_at"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
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

type assignmentCandidateResponse struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type assignmentResponse struct {
	AssignmentID int64  `json:"assignment_id"`
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
}

type assignmentBoardPositionResponse struct {
	Position              publicationPositionResponse   `json:"position"`
	RequiredHeadcount     int                           `json:"required_headcount"`
	Candidates            []assignmentCandidateResponse `json:"candidates"`
	NonCandidateQualified []assignmentCandidateResponse `json:"non_candidate_qualified"`
	Assignments           []assignmentResponse          `json:"assignments"`
}

type assignmentBoardSlotResponse struct {
	Slot      publicationSlotResponse           `json:"slot"`
	Positions []assignmentBoardPositionResponse `json:"positions"`
}

type assignmentBoardResponse struct {
	Publication *publicationResponse          `json:"publication"`
	Slots       []assignmentBoardSlotResponse `json:"slots"`
}

type rosterAssignmentResponse struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type rosterPositionResponse struct {
	Position          publicationPositionResponse `json:"position"`
	RequiredHeadcount int                         `json:"required_headcount"`
	Assignments       []rosterAssignmentResponse  `json:"assignments"`
}

type rosterSlotResponse struct {
	Slot      publicationSlotResponse  `json:"slot"`
	Positions []rosterPositionResponse `json:"positions"`
}

type rosterWeekdayResponse struct {
	Weekday int                  `json:"weekday"`
	Slots   []rosterSlotResponse `json:"slots"`
}

type rosterResponse struct {
	Publication *publicationResponse    `json:"publication"`
	Weekdays    []rosterWeekdayResponse `json:"weekdays"`
}

func newUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:      user.ID,
		Email:   user.Email,
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
		Status:  user.Status,
		Version: user.Version,
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

func newTemplateShiftResponse(shift *model.TemplateShift) templateShiftResponse {
	return templateShiftResponse{
		ID:                shift.ID,
		TemplateID:        shift.TemplateID,
		Weekday:           shift.Weekday,
		StartTime:         shift.StartTime,
		EndTime:           shift.EndTime,
		PositionID:        shift.PositionID,
		RequiredHeadcount: shift.RequiredHeadcount,
		CreatedAt:         shift.CreatedAt,
		UpdatedAt:         shift.UpdatedAt,
	}
}

func newTemplateSlotResponse(slot *model.TemplateSlot) templateSlotResponse {
	response := templateSlotResponse{
		ID:         slot.ID,
		TemplateID: slot.TemplateID,
		Weekday:    slot.Weekday,
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
		ID:                publication.ID,
		TemplateID:        publication.TemplateID,
		TemplateName:      publication.TemplateName,
		Name:              publication.Name,
		State:             publication.State,
		SubmissionStartAt: publication.SubmissionStartAt,
		SubmissionEndAt:   publication.SubmissionEndAt,
		PlannedActiveFrom: publication.PlannedActiveFrom,
		ActivatedAt:       publication.ActivatedAt,
		EndedAt:           publication.EndedAt,
		CreatedAt:         publication.CreatedAt,
		UpdatedAt:         publication.UpdatedAt,
	}
}

func newPublicationSlotResponse(slot *model.TemplateSlot) publicationSlotResponse {
	if slot == nil {
		return publicationSlotResponse{}
	}

	return publicationSlotResponse{
		ID:        slot.ID,
		Weekday:   slot.Weekday,
		StartTime: slot.StartTime,
		EndTime:   slot.EndTime,
	}
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
