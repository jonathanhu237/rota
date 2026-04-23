package model

import (
	"errors"
	"time"
)

var ErrTemplateShiftNotFound = errors.New("template shift not found")

// TemplateShift remains as compatibility scaffolding while publication and
// shift-change flows are still keyed on legacy shift rows.
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
