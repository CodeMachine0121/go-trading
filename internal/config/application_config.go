package config

import (
	"cmp"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// IngestionConfig holds the settings automatic K candle ingestion runs on. The
// interval between rounds is deliberately absent: it is fixed at the length one K
// candle covers, so the way to switch ingestion off is BackgroundJobsEnabled or an
// empty watchlist.
type IngestionConfig struct {
	Symbols                  []string
	RoundCandleCount         int
	BackfillLookback         time.Duration
	MarketDataBaseUrl        string
	MarketDataRequestTimeout time.Duration
}

// LiveFollowConfig holds the three rules a live follow behaves by. All three carry
// a number the requirements name, so they are settings rather than constants: a
// source that behaves differently is a value change, not a code change.
type LiveFollowConfig struct {
	UpdateIntervalCeiling time.Duration
	QuietTimeout          time.Duration
	MaximumRetryDelay     time.Duration
	MarketDataStreamUrl   string
}

// ApplicationConfig holds every setting the binaries read from the environment.
type ApplicationConfig struct {
	ServerPort             string
	CorsAllowedOrigins     []string
	KCandleQueryMaxResults int
	IndicatorScriptTimeout time.Duration
	BackgroundJobsEnabled  bool
	Ingestion              IngestionConfig
	LiveFollow             LiveFollowConfig
	Database               DatabaseConfig
}

// Load reads the configuration from the process environment, applying defaults.
func Load() ApplicationConfig {
	return ApplicationConfig{
		ServerPort: stringWithDefault("SERVER_PORT", "8080"),
		CorsAllowedOrigins: commaSeparatedListWithDefault(
			"CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		KCandleQueryMaxResults: positiveIntWithDefault("KCANDLE_QUERY_MAX_RESULTS", 1000),
		IndicatorScriptTimeout: time.Duration(
			positiveIntWithDefault("INDICATOR_SCRIPT_TIMEOUT_SECONDS", 40)) * time.Second,
		BackgroundJobsEnabled: boolWithDefault("BACKGROUND_JOBS_ENABLED", true),
		Ingestion: IngestionConfig{
			Symbols:          commaSeparatedList("KCANDLE_INGESTION_SYMBOLS"),
			RoundCandleCount: positiveIntWithDefault("KCANDLE_INGESTION_ROUND_CANDLE_COUNT", 5),
			BackfillLookback: time.Duration(
				positiveIntWithDefault("KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", 24)) * time.Hour,
			MarketDataBaseUrl: stringWithDefault(
				"MARKET_DATA_BASE_URL", "https://api.binance.com/api/v3/klines"),
			MarketDataRequestTimeout: time.Duration(
				positiveIntWithDefault("MARKET_DATA_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
		},
		LiveFollow: LiveFollowConfig{
			UpdateIntervalCeiling: time.Duration(
				positiveIntWithDefault("LIVE_UPDATE_INTERVAL_CEILING_SECONDS", 10)) * time.Second,
			QuietTimeout: time.Duration(
				positiveIntWithDefault("LIVE_FEED_QUIET_TIMEOUT_SECONDS", 30)) * time.Second,
			MaximumRetryDelay: time.Duration(
				positiveIntWithDefault("LIVE_FEED_MAX_RETRY_DELAY_SECONDS", 30)) * time.Second,
			MarketDataStreamUrl: stringWithDefault(
				"MARKET_DATA_STREAM_URL", "wss://stream.binance.com:9443/ws"),
		},
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
	return cmp.Or(os.Getenv(key), defaultValue)
}

// boolWithDefault reads a true or false, falling back to the default when the
// variable is missing or says something that is neither.
func boolWithDefault(key string, defaultValue bool) bool {
	value, parseError := strconv.ParseBool(os.Getenv(key))
	if parseError != nil {
		return defaultValue
	}

	return value
}

// commaSeparatedListWithDefault reads a comma separated list, falling back to the
// default when the variable is missing or names nothing.
func commaSeparatedListWithDefault(key string, defaultValue []string) []string {
	entries := commaSeparatedList(key)
	if len(entries) == 0 {
		return defaultValue
	}

	return entries
}

// commaSeparatedList reads a list written as one comma separated line, ignoring
// surrounding spaces and empty entries. A missing variable is an empty list.
func commaSeparatedList(key string) []string {
	entries := make([]string, 0)
	for _, entry := range strings.Split(os.Getenv(key), ",") {
		trimmedEntry := strings.TrimSpace(entry)
		if trimmedEntry != "" {
			entries = append(entries, trimmedEntry)
		}
	}

	return entries
}
