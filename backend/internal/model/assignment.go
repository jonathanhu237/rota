package model

import "time"

type Assignment struct {
	ID              int64
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
	CreatedAt       time.Time
}

type PublicationShift struct {
	ID                int64
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	PositionName      string
	RequiredHeadcount int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AssignmentCandidate struct {
	TemplateShiftID int64
	UserID          int64
	Name            string
	Email           string
}

type AssignmentParticipant struct {
	AssignmentID    int64
	TemplateShiftID int64
	UserID          int64
	Name            string
	Email           string
	CreatedAt       time.Time
}
