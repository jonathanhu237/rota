package model

import "testing"

func TestLeaveStateFromSCRT(t *testing.T) {
	tests := []struct {
		state ShiftChangeState
		want  LeaveState
	}{
		{state: ShiftChangeStatePending, want: LeaveStatePending},
		{state: ShiftChangeStateApproved, want: LeaveStateCompleted},
		{state: ShiftChangeStateExpired, want: LeaveStateFailed},
		{state: ShiftChangeStateRejected, want: LeaveStateFailed},
		{state: ShiftChangeStateCancelled, want: LeaveStateCancelled},
		{state: ShiftChangeStateInvalidated, want: LeaveStateCancelled},
	}

	for _, tt := range tests {
		if got := LeaveStateFromSCRT(tt.state); got != tt.want {
			t.Fatalf("LeaveStateFromSCRT(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
