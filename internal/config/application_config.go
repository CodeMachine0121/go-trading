package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DatabaseConfig holds the PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SslMode  string
}

// DataSourceName builds the PostgreSQL DSN consumed by the GORM driver.
func (databaseConfig DatabaseConfig) DataSourceName() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		databaseConfig.Host,
		databaseConfig.Port,
		databaseConfig.User,
		databaseConfig.Password,
		databaseConfig.Database,
		databaseConfig.SslMode,
	)
}

// ApplicationConfig holds every setting the binaries read from the environment.
type ApplicationConfig struct {
	ServerPort             string
	KCandleQueryMaxResults int
	IndicatorScriptTimeout time.Duration
	Database               DatabaseConfig
}

// Load reads the configuration from the process environment, applying defaults.
func Load() ApplicationConfig {
	return ApplicationConfig{
		ServerPort:             stringWithDefault("SERVER_PORT", "8080"),
		KCandleQueryMaxResults: positiveIntWithDefault("KCANDLE_QUERY_MAX_RESULTS", 1000),
		IndicatorScriptTimeout: time.Duration(
			positiveIntWithDefault("INDICATOR_SCRIPT_TIMEOUT_SECONDS", 40)) * time.Second,
		Database: DatabaseConfig{
			Host:     stringWithDefault("POSTGRES_HOST", "localhost"),
			Port:     stringWithDefault("POSTGRES_PORT", "5432"),
			User:     stringWithDefault("POSTGRES_USER", "postgres"),
			Password: stringWithDefault("POSTGRES_PASSWORD", "postgres"),
			Database: stringWithDefault("POSTGRES_DATABASE", "go_trading"),
			SslMode:  stringWithDefault("POSTGRES_SSL_MODE", "disable"),
		},
	}
}

// positiveIntWithDefault reads a whole number greater than zero, falling back to the
// default when the variable is missing, unreadable, or not a usable count.
func positiveIntWithDefault(key string, defaultValue int) int {
	value, parseError := strconv.Atoi(os.Getenv(key))
	if parseError != nil || value <= 0 {
		return defaultValue
	}

	return value
}

func stringWithDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
