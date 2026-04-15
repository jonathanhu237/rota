package repository

import (
	"database/sql"
	"errors"
	"testing"
)

type scannerStub struct {
	scanErr error
}

func (s scannerStub) Scan(dest ...any) error {
	return s.scanErr
}

func TestSubmissionFromRowReturnsSQLNoRowsUnchanged(t *testing.T) {
	t.Parallel()

	_, err := submissionFromRow(scannerStub{scanErr: sql.ErrNoRows})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}
