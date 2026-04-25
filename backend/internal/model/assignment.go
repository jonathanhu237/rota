package model

import (
	"errors"
	"time"
)

var (
	ErrAssignmentTimeConflict      = errors.New("assignment time conflict")
	ErrAssignmentUserAlreadyInSlot = errors.New("assignment user already in slot")
	ErrSchedulingRetryable         = errors.New("scheduling retryable")
)

type Assignment struct {
	ID            int64
	PublicationID int64
	UserID        int64
	SlotID        int64
	PositionID    int64
	CreatedAt     time.Time
}

type PublicationShift struct {
	ID                int64
	SlotID            int64
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
	SlotID     int64
	PositionID int64
	UserID     int64
	Name       string
	Email      string
}

type AssignmentParticipant struct {
	AssignmentID int64
	SlotID       int64
	PositionID   int64
	UserID       int64
	Name         string
	Email        string
	CreatedAt    time.Time
}

type AssignmentSlotView struct {
	SlotID     int64
	PositionID int64
	Weekday    int
	StartTime  string
	EndTime    string
}
