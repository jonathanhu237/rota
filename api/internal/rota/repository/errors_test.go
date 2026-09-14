package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

func TestPostgresErrorNormalizesPgxAndLibPQ(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		code       string
		constraint string
	}{
		{
			name:       "pgx",
			err:        &pgconn.PgError{Code: "23505", ConstraintName: "positions_name_key"},
			code:       "23505",
			constraint: "positions_name_key",
		},
		{
			name:       "libpq wrapped",
			err:        fmt.Errorf("wrapped: %w", &pq.Error{Code: "23P01", Constraint: "slot_overlap"}),
			code:       "23P01",
			constraint: "slot_overlap",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, constraint, ok := postgresError(test.err)
			if !ok || code != test.code || constraint != test.constraint {
				t.Fatalf("postgresError() = (%q, %q, %v), want (%q, %q, true)", code, constraint, ok, test.code, test.constraint)
			}
		})
	}
}

func TestPostgresErrorIgnoresUnrelatedErrors(t *testing.T) {
	if code, constraint, ok := postgresError(errors.New("database unavailable")); ok || code != "" || constraint != "" {
		t.Fatalf("postgresError() = (%q, %q, %v), want empty false", code, constraint, ok)
	}
}

func TestRepositoryConstraintMappingsSupportPgx(t *testing.T) {
	pgxOverlap := &pgconn.PgError{Code: "23P01"}
	if got := mapTemplateSlotWriteError(pgxOverlap); !errors.Is(got, ErrTemplateSlotOverlap) {
		t.Fatalf("template overlap mapping = %v, want %v", got, ErrTemplateSlotOverlap)
	}

	pgxAssignment := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "assignments_publication_user_slot_weekday_key",
	}
	if got := mapAssignmentWriteError(pgxAssignment); !errors.Is(got, ErrAssignmentUserAlreadyInSlot) {
		t.Fatalf("assignment conflict mapping = %v, want %v", got, ErrAssignmentUserAlreadyInSlot)
	}

	pgxAttendance := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: attendanceRecordsUniqueKey,
	}
	if got := mapAttendanceWriteError(pgxAttendance); !errors.Is(got, ErrAttendanceAlreadyRecorded) {
		t.Fatalf("attendance conflict mapping = %v, want %v", got, ErrAttendanceAlreadyRecorded)
	}
}
