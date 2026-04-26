package config

import (
	"strings"
	"testing"
)

func TestLoadSMTPTLSMode(t *testing.T) {
	tests := []struct {
		name        string
		envMode     string
		tlsMode     string
		wantTLSMode string
		wantErr     string
	}{
		{
			name:        "defaults to starttls",
			envMode:     "smtp",
			wantTLSMode: "starttls",
		},
		{
			name:        "accepts explicit starttls",
			envMode:     "smtp",
			tlsMode:     "starttls",
			wantTLSMode: "starttls",
		},
		{
			name:        "accepts explicit implicit",
			envMode:     "smtp",
			tlsMode:     "implicit",
			wantTLSMode: "implicit",
		},
		{
			name:        "accepts explicit none",
			envMode:     "smtp",
			tlsMode:     "none",
			wantTLSMode: "none",
		},
		{
			name:    "rejects invalid value",
			envMode: "smtp",
			tlsMode: "invalid",
			wantErr: "invalid SMTP_TLS_MODE",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			setTestConfigEnv(t)
			t.Setenv("EMAIL_MODE", tt.envMode)
			if tt.tlsMode != "" {
				t.Setenv("SMTP_TLS_MODE", tt.tlsMode)
			}

			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want %q", tt.wantErr)
				}
				if got := err.Error(); !strings.Contains(got, tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", got, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := cfg.SMTPTLSMode; got != tt.wantTLSMode {
				t.Fatalf("SMTPTLSMode = %q, want %q", got, tt.wantTLSMode)
			}
		})
	}
}

func setTestConfigEnv(t *testing.T) {
	t.Helper()

	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "rota")
	t.Setenv("POSTGRES_PASSWORD", "pa55word")
	t.Setenv("POSTGRES_DB", "rota")
	t.Setenv("SESSION_EXPIRES_HOURS", "336")
	t.Setenv("APP_BASE_URL", "http://localhost:5173")
}
