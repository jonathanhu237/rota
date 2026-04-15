package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidHeadcount      = errors.New("invalid headcount")
	ErrInvalidShiftTime      = errors.New("invalid shift time")
	ErrInvalidWeekday        = errors.New("invalid weekday")
	ErrTemplateLocked        = errors.New("template locked")
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateShiftNotFound = errors.New("template shift not found")
)

type Template struct {
	ID          int64
	Name        string
	Description string
	IsLocked    bool
	ShiftCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Shifts      []*TemplateShift
}

type TemplateShift struct {
	ID                int64
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
