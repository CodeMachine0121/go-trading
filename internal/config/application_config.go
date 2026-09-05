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

// AssistantConfig holds what the market chat assistant runs under: which assistant to
// ask, how hard it may think, and the five ceilings that decide what it may cost.
//
// Every ceiling is a setting rather than a constant because every one of them is a
// number the requirements name — and because what an answer is worth changes with what
// the assistant charges, which is not something this system gets to decide.
//
// They are read through the same reader every other count uses, which refuses zero and
// negatives and falls back to the default. A ceiling of zero would mean an assistant
// that may remember nothing, look at nothing, or spend nothing, and none of those is a
// working system — so refusing the value and carrying on is the honest reading of it.
type AssistantConfig struct {
	ApiKey string
	Model  string
	Effort string
	// BaseUrl is where the assistant is reached. Empty means the assistant's own
	// address, which is what it is in normal use; naming one is how the calls are
	// pointed at a gateway, a recording proxy, or a stand-in during a test.
	BaseUrl string
	// RecentMessageLimit is how many of a conversation's messages the assistant is
	// shown. It is what keeps the cost of an exchange from growing with the length of
	// the conversation.
	RecentMessageLimit int
	// QueryLimit is how many assistant queries one answer may spend.
	QueryLimit int
	// CandleLimit is how many K candles one assistant query may hand over. It is
	// deliberately far below KCandleQueryMaxResults: that one is about what a
	// response can carry, this one about what an answer costs.
	CandleLimit int
	// DailyUsageAllowance is the absolute ceiling on a day's assistant usage. It is
	// the one setting that makes the bill impossible rather than merely unlikely.
	DailyUsageAllowance int
	// AnswerLengthLimit is how long one answer may be.
	AnswerLengthLimit int
	// ResponseTimeout is how long one round trip may take. An assistant that is too
	// slow and one that is unreachable leave the same nothing behind.
	ResponseTimeout time.Duration
}

// AuthenticationConfig holds what recognising a person runs under: the key proofs of
// identity are signed with, and how long one of them lasts.
//
// The key has no default and cannot have one. A default key is a key everybody
// running this code knows, and a proof signed with a key everybody knows is a proof
// anybody can write. Leaving it unset means nobody can sign in — which is the
// correct thing for a system with no key to do, and is why the sign-in path says so
// out loud instead of quietly working in a way that guards nothing.
type AuthenticationConfig struct {
	AccessTokenSigningKey string
	// AccessTokenLifetime is how long the proof every request carries lasts.
	//
	// It is still not stored and so still cannot be taken back — which is why it is
	// now measured in minutes rather than a day. Since sessions became endable, this
	// number is exactly one thing: how long a signed-out access token keeps working.
	AccessTokenLifetime time.Duration
	// RefreshTokenLifetime is how long a renewal proof lasts, counted afresh at every
	// renewal. It is how long somebody may leave the console alone before having to
	// type a password again.
	RefreshTokenLifetime time.Duration
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
	Assistant              AssistantConfig
	Authentication         AuthenticationConfig
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
		Assistant: AssistantConfig{
			ApiKey: stringWithDefault("ANTHROPIC_API_KEY", ""),
			Model:  stringWithDefault("ASSISTANT_MODEL", "claude-opus-5"),
			// Low effort is the right default for a conversation: the thinking that
			// higher effort buys pays off on hard problems, and "how has BTCUSDT
			// moved" is not one. The model itself is left at the capable one, because
			// picking the wrong capability costs more round trips than thinking
			// harder ever saves.
			Effort:              stringWithDefault("ASSISTANT_EFFORT", "low"),
			BaseUrl:             stringWithDefault("ASSISTANT_BASE_URL", ""),
			RecentMessageLimit:  positiveIntWithDefault("ASSISTANT_RECENT_MESSAGE_LIMIT", 20),
			QueryLimit:          positiveIntWithDefault("ASSISTANT_QUERY_LIMIT", 8),
			CandleLimit:         positiveIntWithDefault("ASSISTANT_CANDLE_LIMIT", 200),
			DailyUsageAllowance: positiveIntWithDefault("ASSISTANT_DAILY_USAGE_ALLOWANCE", 300000),
			AnswerLengthLimit:   positiveIntWithDefault("ASSISTANT_ANSWER_LENGTH_LIMIT", 2000),
			ResponseTimeout: time.Duration(
				positiveIntWithDefault("ASSISTANT_RESPONSE_TIMEOUT_SECONDS", 120)) * time.Second,
		},
		Authentication: AuthenticationConfig{
			AccessTokenSigningKey: stringWithDefault("AUTH_ACCESS_TOKEN_SIGNING_KEY", ""),
			// The name says minutes rather than the hours it used to, and the rename
			// is deliberate: the unit changed, and reusing the name would have let an
			// existing setting of 24 mean twenty-four minutes without anybody
			// noticing. An ignored old setting falls back to fifteen minutes, which
			// errs on the strict side.
			AccessTokenLifetime: time.Duration(
				positiveIntWithDefault("AUTH_ACCESS_TOKEN_LIFETIME_MINUTES", 15)) * time.Minute,
			RefreshTokenLifetime: time.Duration(
				positiveIntWithDefault("AUTH_REFRESH_TOKEN_LIFETIME_DAYS", 30)) * 24 * time.Hour,
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
