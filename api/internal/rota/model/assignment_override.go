package model

import "time"

type AssignmentOverride struct {
	ID             int64
	AssignmentID   int64
	OccurrenceDate time.Time
	UserID         string
	CreatedAt      time.Time
}
