package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the application
type Config struct {
	ServerPort             int    `env:"SERVER_PORT,required"`
	PostgresHost           string `env:"POSTGRES_HOST,required"`
	PostgresPort           int    `env:"POSTGRES_PORT,required"`
	PostgresUser           string `env:"POSTGRES_USER,required"`
	PostgresPassword       string `env:"POSTGRES_PASSWORD,required"`
	PostgresDB             string `env:"POSTGRES_DB,required"`
	RedisHost              string `env:"REDIS_HOST,required"`
	RedisPort              int    `env:"REDIS_PORT,required"`
	RedisPassword          string `env:"REDIS_PASSWORD,required"`
	RedisDB                int    `env:"REDIS_DB,required"`
	SessionExpiresHours    int    `env:"SESSION_EXPIRES_HOURS,required"`
	BootstrapAdminEmail    string `env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string `env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string `env:"BOOTSTRAP_ADMIN_NAME"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
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
