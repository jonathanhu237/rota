package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
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
