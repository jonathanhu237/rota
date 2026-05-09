package model

import (
	"errors"
	"time"
)

var (
	ErrAttendanceRecordNotFound  = errors.New("attendance record not found")
	ErrAttendanceNotLeader       = errors.New("attendance not leader")
	ErrAttendanceWindowClosed    = errors.New("attendance window closed")
	ErrAttendanceAlreadyRecorded = errors.New("attendance already recorded")
	ErrAttendanceRosterStale     = errors.New("attendance roster stale")
)

type AttendanceStatus string

const (
	AttendanceStatusPending AttendanceStatus = "pending"
	AttendanceStatusPresent AttendanceStatus = "present"
	AttendanceStatusLate    AttendanceStatus = "late"
	AttendanceStatusAbsent  AttendanceStatus = "absent"
)

type AttendanceRecord struct {
	ID               int64
	PublicationID    int64
	AssignmentID     int64
	OccurrenceDate   time.Time
	UserID           int64
	ArrivedAt        time.Time
	RecordedByUserID *int64
	RecordedAt       time.Time
	UpdatedByUserID  *int64
	UpdatedAt        time.Time
	UserName         string
	UserEmail        string
}

type AttendanceOvertimeRecord struct {
	ID               int64
	PublicationID    int64
	SlotID           int64
	Weekday          int
	OccurrenceDate   time.Time
	UserID           int64
	Hours            float64
	Note             string
	RecordedByUserID *int64
	RecordedAt       time.Time
	UpdatedByUserID  *int64
	UpdatedAt        time.Time
	UserName         string
	UserEmail        string
}

type AttendanceRosterRow struct {
	AssignmentID          int64
	SlotID                int64
	Weekday               int
	PositionID            int64
	PositionName          string
	AttendanceResponsible bool
	UserID                int64
	UserName              string
	UserEmail             string
	CreatedAt             time.Time
	Record                *AttendanceRecord
}

type AttendanceShiftRef struct {
	SlotID         int64
	Weekday        int
	StartTime      string
	EndTime        string
	OccurrenceDate time.Time
}

func DeriveAttendanceStatus(
	arrivedAt *time.Time,
	scheduledStart time.Time,
	scheduledEnd time.Time,
	now time.Time,
) AttendanceStatus {
	if arrivedAt == nil {
		if now.Before(scheduledEnd) {
			return AttendanceStatusPending
		}
		return AttendanceStatusAbsent
	}
	if arrivedAt.After(scheduledStart) {
		return AttendanceStatusLate
	}
	return AttendanceStatusPresent
}
