package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Server    ServerConfig    `envPrefix:"SERVER_"`
	InitAdmin InitAdminConfig `envPrefix:"INIT_ADMIN_"`
	Database  DatabaseConfig  `envPrefix:"DATABASE_"`
}

type ServerConfig struct {
	Port int `env:"PORT"`
}

type InitAdminConfig struct {
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`
	Email    string `env:"EMAIL"`
	Name     string `env:"NAME"`
}

type DatabaseConfig struct {
	User     string `env:"USER"`
	Password string `env:"PASSWORD"`
	Host     string `env:"HOST"`
	Port     int    `env:"PORT"`
	DB       string `env:"DB"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.ParseWithOptions(cfg, env.Options{RequiredIfNoDef: true}); err != nil {
		return nil, err
	}
	return cfg, nil
}
