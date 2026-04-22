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
	ID          int64                   `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	IsLocked    bool                    `json:"is_locked"`
	ShiftCount  int                     `json:"shift_count"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	Shifts      []templateShiftResponse `json:"shifts"`
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

type publicationShiftResponse struct {
	ID                int64     `json:"id"`
	TemplateID        int64     `json:"template_id"`
	Weekday           int       `json:"weekday"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	PositionID        int64     `json:"position_id"`
	PositionName      string    `json:"position_name"`
	RequiredHeadcount int       `json:"required_headcount"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
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

type assignmentBoardShiftResponse struct {
	Shift                 publicationShiftResponse      `json:"shift"`
	Candidates            []assignmentCandidateResponse `json:"candidates"`
	NonCandidateQualified []assignmentCandidateResponse `json:"non_candidate_qualified"`
	Assignments           []assignmentResponse          `json:"assignments"`
}

type assignmentBoardResponse struct {
	Publication *publicationResponse           `json:"publication"`
	Shifts      []assignmentBoardShiftResponse `json:"shifts"`
}

type rosterAssignmentResponse struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type rosterShiftResponse struct {
	Shift       publicationShiftResponse   `json:"shift"`
	Assignments []rosterAssignmentResponse `json:"assignments"`
}

type rosterWeekdayResponse struct {
	Weekday int                   `json:"weekday"`
	Shifts  []rosterShiftResponse `json:"shifts"`
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
		Shifts:      make([]templateShiftResponse, 0, len(template.Shifts)),
	}

	for _, shift := range template.Shifts {
		response.Shifts = append(response.Shifts, newTemplateShiftResponse(shift))
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

func newPublicationShiftResponse(shift *model.PublicationShift) publicationShiftResponse {
	if shift == nil {
		return publicationShiftResponse{}
	}

	return publicationShiftResponse{
		ID:                shift.ID,
		TemplateID:        shift.TemplateID,
		Weekday:           shift.Weekday,
		StartTime:         shift.StartTime,
		EndTime:           shift.EndTime,
		PositionID:        shift.PositionID,
		PositionName:      shift.PositionName,
		RequiredHeadcount: shift.RequiredHeadcount,
		CreatedAt:         shift.CreatedAt,
		UpdatedAt:         shift.UpdatedAt,
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
