package model

import "time"

type AssignmentOverride struct {
	ID             int64
	AssignmentID   int64
	OccurrenceDate time.Time
	UserID         int64
	CreatedAt      time.Time
}
