package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	AssetsDir   string
}

func Load() (*Config, error) {
	// Best-effort: in local dev with `air`, load backend/.env automatically.
	_ = godotenv.Load(".env")

	cfg := &Config{
		Port:        getOrDefault("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AssetsDir:   getOrDefault("ASSETS_DIR", "data"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
