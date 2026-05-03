package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the application
type Config struct {
	AppEnv                 string        `env:"APP_ENV" envDefault:"development"`
	ServerPort             int           `env:"SERVER_PORT,required"`
	PostgresHost           string        `env:"POSTGRES_HOST,required"`
	PostgresPort           int           `env:"POSTGRES_PORT,required"`
	PostgresUser           string        `env:"POSTGRES_USER,required"`
	PostgresPassword       string        `env:"POSTGRES_PASSWORD,required"`
	PostgresDB             string        `env:"POSTGRES_DB,required"`
	SessionExpiresHours    int           `env:"SESSION_EXPIRES_HOURS,required"`
	EmailMode              string        `env:"EMAIL_MODE" envDefault:"log"`
	SMTPHost               string        `env:"SMTP_HOST"`
	SMTPPort               int           `env:"SMTP_PORT" envDefault:"587"`
	SMTPUser               string        `env:"SMTP_USER"`
	SMTPPassword           string        `env:"SMTP_PASSWORD"`
	SMTPFrom               string        `env:"SMTP_FROM"`
	SMTPTLSMode            string        `env:"SMTP_TLS_MODE" envDefault:"starttls"`
	EmailSendTimeout       time.Duration `env:"EMAIL_SEND_TIMEOUT" envDefault:"30s"`
	AppBaseURL             string        `env:"APP_BASE_URL,required"`
	InvitationTokenTTL     time.Duration `env:"INVITATION_TOKEN_TTL" envDefault:"72h"`
	PasswordResetTokenTTL  time.Duration `env:"PASSWORD_RESET_TOKEN_TTL" envDefault:"1h"`
	BootstrapAdminEmail    string        `env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string        `env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string        `env:"BOOTSTRAP_ADMIN_NAME"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	if err := validateSMTPTLSMode(cfg.SMTPTLSMode); err != nil {
		return nil, err
	}
	if err := validateEmailConfig(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateSMTPTLSMode(mode string) error {
	switch mode {
	case "starttls", "implicit", "none":
		return nil
	default:
		return fmt.Errorf("invalid SMTP_TLS_MODE %q: must be one of starttls, implicit, none", mode)
	}
}

func validateEmailConfig(cfg Config) error {
	if cfg.EmailSendTimeout <= 0 {
		return fmt.Errorf("EMAIL_SEND_TIMEOUT must be positive")
	}

	if cfg.EmailMode == "smtp" {
		if strings.TrimSpace(cfg.SMTPHost) == "" {
			return fmt.Errorf("SMTP_HOST is required when EMAIL_MODE=smtp")
		}
		if strings.TrimSpace(cfg.SMTPFrom) == "" {
			return fmt.Errorf("SMTP_FROM is required when EMAIL_MODE=smtp")
		}
	}

	if !strings.EqualFold(cfg.AppEnv, "production") {
		return nil
	}

	if isLocalAppBaseURL(cfg.AppBaseURL) {
		return fmt.Errorf("APP_BASE_URL must be a public URL in production")
	}
	if cfg.SMTPTLSMode == "none" && !isLocalhostOrLoopback(cfg.SMTPHost) {
		return fmt.Errorf("SMTP_TLS_MODE=none is only allowed for localhost SMTP in production")
	}
	return nil
}

func isLocalAppBaseURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return true
	}
	return isLocalhostOrLoopback(parsed.Hostname())
}

func isLocalhostOrLoopback(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" {
		return false
	}
	if splitHost, _, err := net.SplitHostPort(normalized); err == nil {
		normalized = splitHost
	}
	normalized = strings.Trim(normalized, "[]")
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

// DatabaseDSN builds a Postgres DSN from environment values.
func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
	)
}
