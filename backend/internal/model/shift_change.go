package model

import (
	"errors"
	"time"
)

var (
	ErrShiftChangeInvalidType    = errors.New("shift change invalid type")
	ErrShiftChangeNotOwner       = errors.New("shift change not owner")
	ErrShiftChangeNotQualified   = errors.New("shift change not qualified")
	ErrShiftChangeNotPending     = errors.New("shift change not pending")
	ErrShiftChangeExpired        = errors.New("shift change expired")
	ErrShiftChangeInvalidated    = errors.New("shift change invalidated")
	ErrShiftChangeNotFound       = errors.New("shift change not found")
	ErrShiftChangeSelf           = errors.New("shift change self")
	ErrShiftChangeAssignmentMiss = errors.New("shift change assignment missing")
)

// ShiftChangeType discriminates the three request shapes.
type ShiftChangeType string

const (
	ShiftChangeTypeSwap       ShiftChangeType = "swap"
	ShiftChangeTypeGiveDirect ShiftChangeType = "give_direct"
	ShiftChangeTypeGivePool   ShiftChangeType = "give_pool"
)

// ShiftChangeState is the persisted lifecycle state of a request.
type ShiftChangeState string

const (
	ShiftChangeStatePending     ShiftChangeState = "pending"
	ShiftChangeStateApproved    ShiftChangeState = "approved"
	ShiftChangeStateRejected    ShiftChangeState = "rejected"
	ShiftChangeStateCancelled   ShiftChangeState = "cancelled"
	ShiftChangeStateExpired     ShiftChangeState = "expired"
	ShiftChangeStateInvalidated ShiftChangeState = "invalidated"
)

// ShiftChangeRequest is the persisted row.
//
// CounterpartUserID is NULL for give_pool. CounterpartAssignmentID is NULL
// for both give types (there is no counterpart shift; the requester is
// giving away a shift).
type ShiftChangeRequest struct {
	ID                        int64
	PublicationID             int64
	Type                      ShiftChangeType
	RequesterUserID           int64
	RequesterAssignmentID     int64
	OccurrenceDate            time.Time
	CounterpartUserID         *int64
	CounterpartAssignmentID   *int64
	CounterpartOccurrenceDate *time.Time
	State                     ShiftChangeState
	DecidedByUserID           *int64
	CreatedAt                 time.Time
	DecidedAt                 *time.Time
	ExpiresAt                 time.Time
}

// EffectiveShiftChangeState resolves a stored state against the current
// clock. A stored `pending` row whose expires_at has passed is effectively
// expired, but write paths should also persist that on first observation.
func EffectiveShiftChangeState(state ShiftChangeState, expiresAt time.Time, now time.Time) ShiftChangeState {
	if state != ShiftChangeStatePending {
		return state
	}
	if !now.Before(expiresAt) {
		return ShiftChangeStateExpired
	}
	return ShiftChangeStatePending
}
