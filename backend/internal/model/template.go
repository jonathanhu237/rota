package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidHeadcount             = errors.New("invalid headcount")
	ErrInvalidShiftTime             = errors.New("invalid shift time")
	ErrInvalidWeekday               = errors.New("invalid weekday")
	ErrTemplateLocked               = errors.New("template locked")
	ErrTemplateNotFound             = errors.New("template not found")
	ErrTemplateSlotOverlap          = errors.New("template slot overlap")
	ErrTemplateSlotNotFound         = errors.New("template slot not found")
	ErrTemplateSlotPositionNotFound = errors.New("template slot position not found")
)

type Template struct {
	ID          int64
	Name        string
	Description string
	IsLocked    bool
	ShiftCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Slots       []*TemplateSlot
}

type TemplateSlot struct {
	ID         int64
	TemplateID int64
	Weekday    int
	StartTime  string
	EndTime    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Positions  []*TemplateSlotPosition
}

type TemplateSlotPosition struct {
	ID                int64
	SlotID            int64
	PositionID        int64
	RequiredHeadcount int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
