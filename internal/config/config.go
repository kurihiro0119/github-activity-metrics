package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	// GitHub
	GitHubToken string

	// Storage
	StorageType string // "sqlite" or "postgres"
	SQLitePath  string
	PostgresURL string

	// API Server
	APIPort string
	APIHost string

	// CLI
	APIEndpoint string
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	return &Config{
		GitHubToken: getEnv("GITHUB_TOKEN", ""),
		StorageType: getEnv("STORAGE_TYPE", "sqlite"),
		SQLitePath:  getEnv("SQLITE_PATH", "./metrics.db"),
		PostgresURL: getEnv("POSTGRES_URL", ""),
		APIPort:     getEnv("API_PORT", "8080"),
		APIHost:     getEnv("API_HOST", "localhost"),
		APIEndpoint: getEnv("API_ENDPOINT", "http://localhost:8080"),
	}, nil
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GitHubToken == "" {
		return &ConfigError{Field: "GITHUB_TOKEN", Message: "GitHub token is required"}
	}
	if c.StorageType != "sqlite" && c.StorageType != "postgres" {
		return &ConfigError{Field: "STORAGE_TYPE", Message: "must be 'sqlite' or 'postgres'"}
	}
	if c.StorageType == "postgres" && c.PostgresURL == "" {
		return &ConfigError{Field: "POSTGRES_URL", Message: "PostgreSQL URL is required when STORAGE_TYPE is 'postgres'"}
	}
	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
