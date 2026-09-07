package config

import (
	"cmp"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	// A market states its hours in its own zone, and a container built from scratch
	// carries no zone database at all — so a system that reads "Asia/Taipei" perfectly
	// well on a laptop would silently fall back to something else in production.
	// Carrying the database inside the binary costs a few hundred kilobytes and
	// removes the difference.
	_ "time/tzdata"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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
	Symbols           []string
	RoundCandleCount  int
	BackfillLookback  time.Duration
	MarketDataBaseUrl string
	// SymbolCatalogUrl is where a source is asked whether it lists a symbol at all.
	// It is separate from the candle address because they are separate questions, and
	// a source is free to answer them at different places.
	SymbolCatalogUrl         string
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

// TaiwanStockConfig holds what reaching the Taiwan stock market runs on.
//
// Every one of these is a setting rather than a constant because every one of them
// is a fact about a venue and a plan, settled outside this system: where it answers,
// when it trades, and how many of its symbols one plan may follow at once. A venue
// that shifts its hours, or a plan that buys more feeds, is a value change.
type TaiwanStockConfig struct {
	ApiKey string
	// IntradayCandlesUrl answers about today; HistoricalCandlesUrl about any earlier
	// day. They are separate because the source keeps them separate.
	IntradayCandlesUrl   string
	HistoricalCandlesUrl string
	// TickerUrl is where a code is confirmed to exist before it is watched.
	TickerUrl string
	// StreamUrl is where the live feed is opened.
	StreamUrl string
	// TimeZone is the zone this market states its hours in. "Nine o'clock" is a fact
	// about Taipei, and the moment it names universally is not the same one all year
	// in markets that shift with daylight saving.
	TimeZone *time.Location
	// SessionStart and SessionEnd are how far into its local day trading begins and
	// ends.
	SessionStart time.Duration
	SessionEnd   time.Duration
	// SimultaneousFollowCeiling is how many of this market's symbols may be followed
	// live at once, as the market data plan allows.
	SimultaneousFollowCeiling int
	RequestTimeout            time.Duration
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
	TaiwanStock            TaiwanStockConfig
	// MarketRules is how every market the system recognises behaves. Recognising one
	// more market is one more entry here and two more sources wired to it; nothing
	// inside the system branches on which market it is looking at.
	MarketRules    map[vo.MarketVo]vo.MarketRulesVo
	Assistant      AssistantConfig
	Authentication AuthenticationConfig
	Database       DatabaseConfig
}

// Load reads the configuration from the process environment, applying defaults.
func Load() ApplicationConfig {
	taiwanStockConfig := loadTaiwanStockConfig()

	return ApplicationConfig{
		ServerPort: stringWithDefault("SERVER_PORT", "8080"),
		CorsAllowedOrigins: commaSeparatedListWithDefault(
			"CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		KCandleQueryMaxResults: positiveIntWithDefault("KCANDLE_QUERY_MAX_RESULTS", 1000),
		IndicatorScriptTimeout: time.Duration(
			positiveIntWithDefault("INDICATOR_SCRIPT_TIMEOUT_SECONDS", 40)) * time.Second,
		BackgroundJobsEnabled: boolWithDefault("BACKGROUND_JOBS_ENABLED", true),
		TaiwanStock:           taiwanStockConfig,
		MarketRules:           marketRules(taiwanStockConfig),
		Ingestion: IngestionConfig{
			RoundCandleCount: positiveIntWithDefault("KCANDLE_INGESTION_ROUND_CANDLE_COUNT", 5),
			BackfillLookback: time.Duration(
				positiveIntWithDefault("KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", 24)) * time.Hour,
			MarketDataBaseUrl: stringWithDefault(
				"MARKET_DATA_BASE_URL", "https://api.binance.com/api/v3/klines"),
			SymbolCatalogUrl: stringWithDefault(
				"MARKET_DATA_SYMBOL_CATALOG_URL", "https://api.binance.com/api/v3/exchangeInfo"),
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

// marketRules is every market the system recognises and how each one behaves.
//
// A market that never closes and has no follow ceiling is written as the zero value
// rather than as a special case, so that the rules read it exactly as they read any
// other market.
//
// Recognising a third market is one more entry here and its sources wired up beside
// the others; nothing inside the system branches on which market it is looking at.
func marketRules(taiwanStockConfig TaiwanStockConfig) map[vo.MarketVo]vo.MarketRulesVo {
	return map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   taiwanStockConfig.TimeZone,
				DailyStart: taiwanStockConfig.SessionStart,
				DailyEnd:   taiwanStockConfig.SessionEnd,
				// Weekends are not a setting. Every stock exchange takes them off, and
				// the days it additionally takes off are read from its own answers
				// rather than kept in a list somebody has to maintain.
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			SimultaneousFollowCeiling: taiwanStockConfig.SimultaneousFollowCeiling,
		},
	}
}

func loadTaiwanStockConfig() TaiwanStockConfig {
	return TaiwanStockConfig{
		ApiKey: stringWithDefault("TAIWAN_STOCK_API_KEY", ""),
		IntradayCandlesUrl: stringWithDefault("TAIWAN_STOCK_INTRADAY_CANDLES_URL",
			"https://api.fugle.tw/marketdata/v1.0/stock/intraday/candles"),
		HistoricalCandlesUrl: stringWithDefault("TAIWAN_STOCK_HISTORICAL_CANDLES_URL",
			"https://api.fugle.tw/marketdata/v1.0/stock/historical/candles"),
		TickerUrl: stringWithDefault("TAIWAN_STOCK_TICKER_URL",
			"https://api.fugle.tw/marketdata/v1.0/stock/intraday/ticker"),
		StreamUrl: stringWithDefault("TAIWAN_STOCK_STREAM_URL",
			"wss://api.fugle.tw/marketdata/v1.0/stock/streaming"),
		TimeZone:     timeZoneWithDefault("TAIWAN_STOCK_TIME_ZONE", "Asia/Taipei"),
		SessionStart: timeOfDayWithDefault("TAIWAN_STOCK_SESSION_START", 9*time.Hour),
		SessionEnd:   timeOfDayWithDefault("TAIWAN_STOCK_SESSION_END", 13*time.Hour+30*time.Minute),
		SimultaneousFollowCeiling: positiveIntWithDefault(
			"TAIWAN_STOCK_SIMULTANEOUS_FOLLOW_CEILING", 5),
		RequestTimeout: time.Duration(
			positiveIntWithDefault("TAIWAN_STOCK_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
	}
}

// timeZoneWithDefault reads a zone by name, falling back to the default when the
// variable is missing or names a zone this machine has never heard of.
//
// A zone that cannot be loaded must not leave the market with none: a nil zone is how
// a market says it never closes, so a typo here would quietly turn a market that
// shuts every evening into one that never does.
func timeZoneWithDefault(key string, defaultName string) *time.Location {
	location, loadError := time.LoadLocation(stringWithDefault(key, defaultName))
	if loadError != nil {
		location, loadError = time.LoadLocation(defaultName)
		if loadError != nil {
			return time.UTC
		}
	}

	return location
}

// timeOfDayWithDefault reads a time of day written as HH:MM and hands back how far
// into the day it is, falling back to the default when the variable is missing or
// says something that is not a time of day.
func timeOfDayWithDefault(key string, defaultValue time.Duration) time.Duration {
	timeOfDay, parseError := time.Parse("15:04", os.Getenv(key))
	if parseError != nil {
		return defaultValue
	}

	return time.Duration(timeOfDay.Hour())*time.Hour + time.Duration(timeOfDay.Minute())*time.Minute
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
