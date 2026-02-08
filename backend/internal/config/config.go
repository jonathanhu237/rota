package config

import (
	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the application
type Config struct {
	ServerPort int `env:"SERVER_PORT" envDefault:"8080"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
