package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogInsecureSMTPTLSWarning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		emailMode   string
		tlsMode     string
		wantWarning bool
	}{
		{
			name:        "warns for smtp without tls",
			emailMode:   "smtp",
			tlsMode:     "none",
			wantWarning: true,
		},
		{
			name:      "does not warn for starttls",
			emailMode: "smtp",
			tlsMode:   "starttls",
		},
		{
			name:      "does not warn for logger emailer",
			emailMode: "log",
			tlsMode:   "none",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))

			logInsecureSMTPTLSWarning(logger, tt.emailMode, tt.tlsMode)

			written := output.String()
			if tt.wantWarning {
				if !strings.Contains(written, "SMTP is configured without TLS") {
					t.Fatalf("expected warning message, got %q", written)
				}
				if !strings.Contains(written, "tls_mode=none") {
					t.Fatalf("expected tls_mode field, got %q", written)
				}
				return
			}

			if written != "" {
				t.Fatalf("expected no warning output, got %q", written)
			}
		})
	}
}

func TestStartSessionCleanupContinuesAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := &sessionCleanupDBStub{
		errs: []error{errors.New("transient db error"), nil},
		done: make(chan struct{}),
	}

	startSessionCleanup(ctx, db, time.Millisecond, slog.New(slog.DiscardHandler))

	select {
	case <-db.done:
	case <-time.After(time.Second):
		t.Fatalf("cleanup did not continue after first error")
	}

	if got := db.lastQuery(); got != `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day';` {
		t.Fatalf("cleanup query = %q", got)
	}
}

type sessionCleanupDBStub struct {
	mu       sync.Mutex
	errs     []error
	calls    int
	query    string
	doneOnce sync.Once
	done     chan struct{}
}

func (s *sessionCleanupDBStub) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	s.query = query

	var err error
	if s.calls <= len(s.errs) {
		err = s.errs[s.calls-1]
	}
	if s.calls >= 2 {
		s.doneOnce.Do(func() { close(s.done) })
	}
	return sessionCleanupResult{}, err
}

func (s *sessionCleanupDBStub) lastQuery() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.query
}

type sessionCleanupResult struct{}

func (sessionCleanupResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (sessionCleanupResult) RowsAffected() (int64, error) {
	return 0, nil
}
