package model

import (
	"testing"
	"time"
)

func TestDeriveAttendanceStatus(t *testing.T) {
	start := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		arrivedAt *time.Time
		now       time.Time
		want      AttendanceStatus
	}{
		{
			name: "pending before end without arrival",
			now:  start.Add(2 * time.Hour),
			want: AttendanceStatusPending,
		},
		{
			name: "absent at end without arrival",
			now:  end,
			want: AttendanceStatusAbsent,
		},
		{
			name:      "present at scheduled start",
			arrivedAt: ptrTime(start),
			now:       start.Add(30 * time.Minute),
			want:      AttendanceStatusPresent,
		},
		{
			name:      "late after scheduled start",
			arrivedAt: ptrTime(start.Add(15 * time.Minute)),
			now:       start.Add(30 * time.Minute),
			want:      AttendanceStatusLate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveAttendanceStatus(tt.arrivedAt, start, end, tt.now); got != tt.want {
				t.Fatalf("DeriveAttendanceStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
