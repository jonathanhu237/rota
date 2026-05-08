package model

import (
	"errors"
	"time"
)

var (
	ErrLeaveNotFound = errors.New("leave not found")
	ErrLeaveNotOwner = errors.New("leave not owner")
)

type LeaveCategory string

const (
	LeaveCategorySick        LeaveCategory = "sick"
	LeaveCategoryPersonal    LeaveCategory = "personal"
	LeaveCategoryBereavement LeaveCategory = "bereavement"
)

type LeaveState string

const (
	LeaveStatePending   LeaveState = "pending"
	LeaveStateCompleted LeaveState = "completed"
	LeaveStateFailed    LeaveState = "failed"
	LeaveStateCancelled LeaveState = "cancelled"
)

type LeavePoolStateFilter string

const (
	LeavePoolStatePending   LeavePoolStateFilter = "pending"
	LeavePoolStateCompleted LeavePoolStateFilter = "completed"
	LeavePoolStateFailed    LeavePoolStateFilter = "failed"
	LeavePoolStateCancelled LeavePoolStateFilter = "cancelled"
	LeavePoolStateAll       LeavePoolStateFilter = "all"
)

type LeaveActionDisabledReason string

const (
	LeaveActionDisabledNone          LeaveActionDisabledReason = ""
	LeaveActionDisabledNotQualified  LeaveActionDisabledReason = "not_qualified"
	LeaveActionDisabledAdminViewOnly LeaveActionDisabledReason = "admin_view_only"
)

type Leave struct {
	ID                   int64
	UserID               int64
	PublicationID        int64
	ShiftChangeRequestID int64
	Category             LeaveCategory
	Reason               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func LeaveStateFromSCRT(state ShiftChangeState) LeaveState {
	switch state {
	case ShiftChangeStateApproved:
		return LeaveStateCompleted
	case ShiftChangeStateExpired, ShiftChangeStateRejected:
		return LeaveStateFailed
	case ShiftChangeStateCancelled, ShiftChangeStateInvalidated:
		return LeaveStateCancelled
	default:
		return LeaveStatePending
	}
}
