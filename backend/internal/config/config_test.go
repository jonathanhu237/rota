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
			if tt.envMode == "smtp" {
				t.Setenv("SMTP_HOST", "smtp.example.com")
				t.Setenv("SMTP_FROM", "Rota <noreply@example.com>")
			}
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

func TestLoadEmailSendTimeout(t *testing.T) {
	t.Run("defaults to thirty seconds", func(t *testing.T) {
		setTestConfigEnv(t)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.EmailSendTimeout.String() != "30s" {
			t.Fatalf("EmailSendTimeout = %s, want 30s", cfg.EmailSendTimeout)
		}
	})

	t.Run("rejects non-positive duration", func(t *testing.T) {
		setTestConfigEnv(t)
		t.Setenv("EMAIL_SEND_TIMEOUT", "0s")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "EMAIL_SEND_TIMEOUT") {
			t.Fatalf("Load() error = %v, want EMAIL_SEND_TIMEOUT error", err)
		}
	})
}

func TestLoadEmailConfigurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T)
		wantErr string
	}{
		{
			name: "smtp requires host",
			mutate: func(t *testing.T) {
				t.Setenv("EMAIL_MODE", "smtp")
				t.Setenv("SMTP_HOST", "")
				t.Setenv("SMTP_FROM", "Rota <noreply@example.com>")
			},
			wantErr: "SMTP_HOST",
		},
		{
			name: "smtp requires sender",
			mutate: func(t *testing.T) {
				t.Setenv("EMAIL_MODE", "smtp")
				t.Setenv("SMTP_HOST", "smtp.example.com")
				t.Setenv("SMTP_FROM", "")
			},
			wantErr: "SMTP_FROM",
		},
		{
			name: "production rejects localhost app url",
			mutate: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("APP_BASE_URL", "http://localhost:5173")
			},
			wantErr: "APP_BASE_URL",
		},
		{
			name: "production rejects insecure remote smtp",
			mutate: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("APP_BASE_URL", "https://rota.example.com")
				t.Setenv("SMTP_TLS_MODE", "none")
				t.Setenv("SMTP_HOST", "smtp.example.com")
			},
			wantErr: "SMTP_TLS_MODE=none",
		},
		{
			name: "production allows insecure localhost smtp",
			mutate: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("APP_BASE_URL", "https://rota.example.com")
				t.Setenv("SMTP_TLS_MODE", "none")
				t.Setenv("SMTP_HOST", "localhost")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			setTestConfigEnv(t)
			tt.mutate(t)

			_, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
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
