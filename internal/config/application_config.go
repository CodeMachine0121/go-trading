package config

import (
	"cmp"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	// Embedded zone database, since scratch containers have none and "Asia/Taipei" would silently
	// fail to load.
	_ "time/tzdata"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SslMode  string
}

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

// IngestionConfig has no round interval because it is fixed at one K candle's length; disable
// ingestion via BackgroundJobsEnabled or an empty watchlist.
type IngestionConfig struct {
	Symbols          []string
	RoundCandleCount int
	BackfillLookback time.Duration
	// HistorySyncMaxLookbackDays is only a typo guard (ten years exceeds any source's history),
	// since syncs now stream chunk by chunk.
	HistorySyncMaxLookbackDays int
	MarketDataBaseUrl          string
	SymbolCatalogUrl           string
	MarketDataRequestTimeout   time.Duration
	// MarketDataRequestsPerMinute is configurable because it is the venue's allowance, which can
	// change.
	MarketDataRequestsPerMinute int
}

// ContractIngestionConfig mirrors the spot settings without sharing any: the venues budget requests
// separately and each contract candle costs two requests.
type ContractIngestionConfig struct {
	RoundCandleCount           int
	BackfillLookback           time.Duration
	HistorySyncMaxLookbackDays int
	// BaseUrl serves traded figures; the other three serve mark, index and premium index prices,
	// all combined into one contract candle.
	BaseUrl         string
	MarkPriceUrl    string
	IndexPriceUrl   string
	PremiumIndexUrl string
	// SymbolCatalogUrl returns the venue's whole catalogue regardless of the query.
	SymbolCatalogUrl string
	// FundingInfoUrl lists contracts whose funding rate settles on a non-default interval.
	FundingInfoUrl string
	// FundingRateUrl shares the candle request allowance.
	FundingRateUrl    string
	RequestTimeout    time.Duration
	RequestsPerMinute int
	// StatisticsBaseUrl is rate-limited by the venue separately from everything else, hence its own
	// allowance.
	StatisticsBaseUrl           string
	StatisticsRequestsPerMinute int
	// PositionStatisticArchiveBaseUrl is a different host holding years of daily files, where the
	// live endpoints keep only thirty days.
	PositionStatisticArchiveBaseUrl           string
	PositionStatisticArchiveRequestsPerMinute int
	// Zero disables the respective round.
	FundingRateIngestionInterval        time.Duration
	PositionStatisticIngestionInterval  time.Duration
	TradingSpecificationRefreshInterval time.Duration
	// AccountApiKey and AccountApiSecret are read-only credentials needed only for the maintenance
	// margin ladder; they have no default, and when empty the ladder is skipped.
	AccountApiKey                        string
	AccountApiSecret                     string
	MaintenanceMarginTierUrl             string
	MaintenanceMarginTierRefreshInterval time.Duration
}

type LiveFollowConfig struct {
	UpdateIntervalCeiling time.Duration
	QuietTimeout          time.Duration
	MaximumRetryDelay     time.Duration
	MarketDataStreamUrl   string
	// ContractMarketDataStreamUrl is the only contract-specific live setting; the timing rules are
	// market-independent.
	ContractMarketDataStreamUrl string
}

type TaiwanStockConfig struct {
	ApiKey               string
	IntradayCandlesUrl   string
	HistoricalCandlesUrl string
	TickerUrl            string
	// LiveCandleInterval shares RequestsPerMinute with history fetches, and since one request
	// covers one symbol the proxy uses whichever of the two is slower.
	LiveCandleInterval time.Duration
	// TimeZone is the zone the session hours are stated in, which matters under daylight saving.
	TimeZone *time.Location
	// SessionStart and SessionEnd are offsets into the local day.
	SessionStart time.Duration
	SessionEnd   time.Duration
	// SymbolsPerLiveChannel is symbols per request round, not a subscription limit; each symbol is
	// queried under both boards, so requests carry twice this many entries.
	SymbolsPerLiveChannel int
	RequestTimeout        time.Duration
	// RequestsPerMinute matters because this source answers one local day per request, so
	// multi-year syncs take thousands of requests.
	RequestsPerMinute int
}

// AssistantConfig ceilings reject zero or negatives and fall back to defaults, since a zero ceiling
// would make the assistant unusable.
type AssistantConfig struct {
	ApiKey string
	Model  string
	Effort string
	// BaseUrl empty means the provider's default; set it to route through a gateway or a test
	// stand-in.
	BaseUrl string
	// RecentMessageLimit bounds how many messages the assistant sees, keeping per-exchange cost
	// flat.
	RecentMessageLimit int
	// QueryLimit is high because one answer may iterate build-replay-adjust loops of about five
	// queries each.
	QueryLimit int
	// CandleLimit bounds per-query cost and is deliberately far below KCandleQueryMaxResults.
	CandleLimit int
	// DailyUsageAllowance is the hard daily cap that bounds the bill.
	DailyUsageAllowance int
	AnswerLengthLimit   int
	ResponseTimeout     time.Duration
}

// AuthenticationConfig's signing key has no default, because a shared default key would let anyone
// forge tokens; unset means sign-in is refused.
type AuthenticationConfig struct {
	AccessTokenSigningKey string
	// AccessTokenLifetime is short because access tokens cannot be revoked, so it bounds how long a
	// signed-out token keeps working.
	AccessTokenLifetime time.Duration
	// RefreshTokenLifetime restarts at every renewal.
	RefreshTokenLifetime time.Duration
}

// AccountActivationConfig has defaults because neither value is a secret.
type AccountActivationConfig struct {
	RequestMailbox string
	// SubjectPrefix is followed by the applicant's address so the inbox reads as a list of
	// applicants.
	SubjectPrefix string
}

// SignInLockoutConfig is configurable mainly so tests need not wait out a week-long lock.
type SignInLockoutConfig struct {
	FailureThreshold int
	LockoutDuration  time.Duration
}

// SecretsConfig's key has no default, because a shared default key would leave secrets effectively
// in the open; unset means no secret can be stored.
type SecretsConfig struct {
	// SealKey is 32 bytes base64-encoded; any other length counts as no key rather than being
	// padded or truncated.
	SealKey string
}

type TelegramConfig struct {
	ApiBaseUrl     string
	RequestTimeout time.Duration
}

// StrategyBotConfig holds deployment tuning; limits that define bot semantics live in the domain.
type StrategyBotConfig struct {
	// ScanInterval matches the shortest bot trigger interval.
	ScanInterval time.Duration
	// MaxConcurrentRounds caps both how many due bots one scan loads and how many run in parallel.
	MaxConcurrentRounds int
	RoundTimeout        time.Duration
}

type ApplicationConfig struct {
	ServerPort             string
	CorsAllowedOrigins     []string
	KCandleQueryMaxResults int
	IndicatorScriptTimeout time.Duration
	// IndicatorScriptMemoryLimitBytes applies per script compartment and must sit well below the
	// service's own memory, as several run concurrently.
	IndicatorScriptMemoryLimitBytes int64
	// IndicatorScriptMaxConcurrentCompartments times the memory limit must leave room for the service.
	IndicatorScriptMaxConcurrentCompartments int
	// BacktestMaxCandleCount limits buckets per replay, separately from the single-query ceiling.
	BacktestMaxCandleCount int
	// BacktestTimeAllowance covers the whole replay and sits below the common 100s reverse-proxy
	// timeout so the replay reports a timeout itself.
	BacktestTimeAllowance time.Duration
	BackgroundJobsEnabled bool
	Ingestion             IngestionConfig
	ContractIngestion     ContractIngestionConfig
	LiveFollow            LiveFollowConfig
	TaiwanStock           TaiwanStockConfig
	// MarketRules maps each market to its behaviour so nothing branches on market identity.
	MarketRules       map[vo.MarketVo]vo.MarketRulesVo
	Assistant         AssistantConfig
	Authentication    AuthenticationConfig
	AccountActivation AccountActivationConfig
	SignInLockout     SignInLockoutConfig
	Secrets           SecretsConfig
	Telegram          TelegramConfig
	StrategyBot       StrategyBotConfig
	Database          DatabaseConfig
}

func Load() ApplicationConfig {
	taiwanStockConfig := loadTaiwanStockConfig()

	return ApplicationConfig{
		ServerPort: stringWithDefault("SERVER_PORT", "8080"),
		CorsAllowedOrigins: commaSeparatedListWithDefault(
			"CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		KCandleQueryMaxResults: positiveIntWithDefault("KCANDLE_QUERY_MAX_RESULTS", 1000),
		IndicatorScriptTimeout: time.Duration(
			positiveIntWithDefault("INDICATOR_SCRIPT_TIMEOUT_SECONDS", 40)) * time.Second,
		IndicatorScriptMemoryLimitBytes: int64(
			positiveIntWithDefault("INDICATOR_SCRIPT_MEMORY_LIMIT_MEGABYTES", 512)) << 20,
		// Six 512MB compartments fill 3GB of the production 4GB, leaving 1GB for the service.
		IndicatorScriptMaxConcurrentCompartments: positiveIntWithDefault(
			"INDICATOR_SCRIPT_MAX_CONCURRENT_COMPARTMENTS", 6),
		BacktestMaxCandleCount: positiveIntWithDefault("BACKTEST_MAX_CANDLE_COUNT", 50000),
		BacktestTimeAllowance: time.Duration(
			positiveIntWithDefault("BACKTEST_TIME_ALLOWANCE_SECONDS", 90)) * time.Second,
		BackgroundJobsEnabled: boolWithDefault("BACKGROUND_JOBS_ENABLED", true),
		TaiwanStock:           taiwanStockConfig,
		MarketRules:           marketRules(taiwanStockConfig),
		Ingestion: IngestionConfig{
			RoundCandleCount: positiveIntWithDefault("KCANDLE_INGESTION_ROUND_CANDLE_COUNT", 25),
			BackfillLookback: time.Duration(
				positiveIntWithDefault("KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", 24)) * time.Hour,
			HistorySyncMaxLookbackDays: positiveIntWithDefault(
				"KCANDLE_HISTORY_SYNC_MAX_LOOKBACK_DAYS", 3650),
			MarketDataBaseUrl: stringWithDefault(
				"MARKET_DATA_BASE_URL", "https://api.binance.com/api/v3/klines"),
			SymbolCatalogUrl: stringWithDefault(
				"MARKET_DATA_SYMBOL_CATALOG_URL", "https://api.binance.com/api/v3/exchangeInfo"),
			MarketDataRequestTimeout: time.Duration(
				positiveIntWithDefault("MARKET_DATA_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
			// Deliberately below the venue's limit, since the allowance is shared with other
			// requests and throttling costs more than slowness.
			MarketDataRequestsPerMinute: positiveIntWithDefault(
				"MARKET_DATA_REQUESTS_PER_MINUTE", 600),
		},
		ContractIngestion: ContractIngestionConfig{
			RoundCandleCount: positiveIntWithDefault(
				"CONTRACT_KCANDLE_INGESTION_ROUND_CANDLE_COUNT", 25),
			BackfillLookback: time.Duration(positiveIntWithDefault(
				"CONTRACT_KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", 24)) * time.Hour,
			HistorySyncMaxLookbackDays: positiveIntWithDefault(
				"CONTRACT_KCANDLE_HISTORY_SYNC_MAX_LOOKBACK_DAYS", 3650),
			BaseUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_BASE_URL", "https://fapi.binance.com/fapi/v1/klines"),
			MarkPriceUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_MARK_PRICE_URL",
				"https://fapi.binance.com/fapi/v1/markPriceKlines"),
			IndexPriceUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_INDEX_PRICE_URL",
				"https://fapi.binance.com/fapi/v1/indexPriceKlines"),
			PremiumIndexUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_PREMIUM_INDEX_URL",
				"https://fapi.binance.com/fapi/v1/premiumIndexKlines"),
			SymbolCatalogUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_SYMBOL_CATALOG_URL",
				"https://fapi.binance.com/fapi/v1/exchangeInfo"),
			FundingInfoUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_FUNDING_INFO_URL",
				"https://fapi.binance.com/fapi/v1/fundingInfo"),
			FundingRateUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_FUNDING_RATE_URL",
				"https://fapi.binance.com/fapi/v1/fundingRate"),
			StatisticsBaseUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_STATISTICS_BASE_URL",
				"https://fapi.binance.com/futures/data"),
			// Slightly under the venue's 1000-per-5-minutes limit so a thirty-day catch-up and the
			// five-minute round never hit it together.
			StatisticsRequestsPerMinute: positiveIntWithDefault(
				"CONTRACT_MARKET_DATA_STATISTICS_REQUESTS_PER_MINUTE", 180),
			PositionStatisticArchiveBaseUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_BASE_URL",
				"https://data.binance.vision/data/futures/um/daily/metrics"),
			// The archive host publishes no limit; two per second is already far faster than candle
			// syncs.
			PositionStatisticArchiveRequestsPerMinute: positiveIntWithDefault(
				"CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_REQUESTS_PER_MINUTE", 120),
			FundingRateIngestionInterval: jobIntervalWithDefault(
				"CONTRACT_FUNDING_RATE_INGESTION_INTERVAL_MINUTES", 60, time.Minute),
			PositionStatisticIngestionInterval: jobIntervalWithDefault(
				"CONTRACT_POSITION_STATISTIC_INGESTION_INTERVAL_MINUTES", 5, time.Minute),
			TradingSpecificationRefreshInterval: jobIntervalWithDefault(
				"CONTRACT_TRADING_SPECIFICATION_REFRESH_INTERVAL_HOURS", 24, time.Hour),
			AccountApiKey:    os.Getenv("CONTRACT_ACCOUNT_API_KEY"),
			AccountApiSecret: os.Getenv("CONTRACT_ACCOUNT_API_SECRET"),
			MaintenanceMarginTierUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_MAINTENANCE_MARGIN_TIER_URL",
				"https://fapi.binance.com/fapi/v1/leverageBracket"),
			MaintenanceMarginTierRefreshInterval: jobIntervalWithDefault(
				"CONTRACT_MAINTENANCE_MARGIN_TIER_REFRESH_INTERVAL_HOURS", 24, time.Hour),
			RequestTimeout: time.Duration(positiveIntWithDefault(
				"CONTRACT_MARKET_DATA_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
			// Half the spot allowance because each contract candle costs two requests.
			RequestsPerMinute: positiveIntWithDefault(
				"CONTRACT_MARKET_DATA_REQUESTS_PER_MINUTE", 300),
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
			ContractMarketDataStreamUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_STREAM_URL", "wss://fstream.binance.com/ws"),
		},
		Assistant: AssistantConfig{
			ApiKey: stringWithDefault("ANTHROPIC_API_KEY", ""),
			Model:  stringWithDefault("ASSISTANT_MODEL", "claude-opus-5"),
			// Low effort suits conversational questions, while the model stays the capable one
			// since wrong capability costs more round trips.
			Effort:              stringWithDefault("ASSISTANT_EFFORT", "low"),
			BaseUrl:             stringWithDefault("ASSISTANT_BASE_URL", ""),
			RecentMessageLimit:  positiveIntWithDefault("ASSISTANT_RECENT_MESSAGE_LIMIT", 20),
			QueryLimit:          positiveIntWithDefault("ASSISTANT_QUERY_LIMIT", 40),
			CandleLimit:         positiveIntWithDefault("ASSISTANT_CANDLE_LIMIT", 200),
			DailyUsageAllowance: positiveIntWithDefault("ASSISTANT_DAILY_USAGE_ALLOWANCE", 300000),
			AnswerLengthLimit:   positiveIntWithDefault("ASSISTANT_ANSWER_LENGTH_LIMIT", 2000),
			ResponseTimeout: time.Duration(
				positiveIntWithDefault("ASSISTANT_RESPONSE_TIMEOUT_SECONDS", 120)) * time.Second,
		},
		Authentication: AuthenticationConfig{
			AccessTokenSigningKey: stringWithDefault("AUTH_ACCESS_TOKEN_SIGNING_KEY", ""),
			// Named MINUTES (not the old HOURS) so an existing value of 24 is ignored rather than
			// silently reinterpreted.
			AccessTokenLifetime: time.Duration(
				positiveIntWithDefault("AUTH_ACCESS_TOKEN_LIFETIME_MINUTES", 15)) * time.Minute,
			RefreshTokenLifetime: time.Duration(
				positiveIntWithDefault("AUTH_REFRESH_TOKEN_LIFETIME_DAYS", 30)) * 24 * time.Hour,
		},
		AccountActivation: AccountActivationConfig{
			RequestMailbox: stringWithDefault(
				"ACCOUNT_ACTIVATION_REQUEST_MAILBOX", "james.afternoon.dev@gmail.com"),
			SubjectPrefix: stringWithDefault(
				"ACCOUNT_ACTIVATION_SUBJECT_PREFIX", "go-trading 開通申請"),
		},
		SignInLockout: SignInLockoutConfig{
			FailureThreshold: positiveIntWithDefault("AUTH_SIGN_IN_FAILURE_THRESHOLD", 3),
			// A week is deliberate: short locks barely slow automated guessing, and with few users
			// an unlock is just one edited row.
			LockoutDuration: time.Duration(
				positiveIntWithDefault("AUTH_SIGN_IN_LOCKOUT_DAYS", 7)) * 24 * time.Hour,
		},
		Secrets: SecretsConfig{
			SealKey: stringWithDefault("SECRET_SEAL_KEY", ""),
		},
		Telegram: TelegramConfig{
			ApiBaseUrl: stringWithDefault("TELEGRAM_API_BASE_URL", "https://api.telegram.org"),
			RequestTimeout: time.Duration(
				positiveIntWithDefault("TELEGRAM_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
		},
		StrategyBot: StrategyBotConfig{
			ScanInterval: time.Duration(
				positiveIntWithDefault("STRATEGY_BOT_SCAN_INTERVAL_SECONDS", 60)) * time.Second,
			MaxConcurrentRounds: positiveIntWithDefault("STRATEGY_BOT_MAX_CONCURRENT_ROUNDS", 4),
			RoundTimeout: time.Duration(
				positiveIntWithDefault("STRATEGY_BOT_ROUND_TIMEOUT_SECONDS", 120)) * time.Second,
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

// marketRules writes a never-closing, uncapped market as the zero value so it needs no special
// case.
func marketRules(taiwanStockConfig TaiwanStockConfig) map[vo.MarketVo]vo.MarketRulesVo {
	return map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   taiwanStockConfig.TimeZone,
				DailyStart: taiwanStockConfig.SessionStart,
				DailyEnd:   taiwanStockConfig.SessionEnd,
				// Weekends are fixed; exchange holidays are inferred from the venue's own answers.
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			// Taiwan follows a fixed roster of watched stocks regardless of viewers; the
			// per-channel cap stays zero (uncapped) because the exchange sells no subscriptions.
			FollowsFixedRoster:    true,
			SymbolsPerLiveChannel: taiwanStockConfig.SymbolsPerLiveChannel,
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
		LiveCandleInterval: time.Duration(positiveIntWithDefault(
			"TAIWAN_STOCK_LIVE_CANDLE_INTERVAL_SECONDS", 5)) * time.Second,
		TimeZone:     timeZoneWithDefault("TAIWAN_STOCK_TIME_ZONE", "Asia/Taipei"),
		SessionStart: timeOfDayWithDefault("TAIWAN_STOCK_SESSION_START", 9*time.Hour),
		SessionEnd:   timeOfDayWithDefault("TAIWAN_STOCK_SESSION_END", 13*time.Hour+30*time.Minute),
		SymbolsPerLiveChannel: positiveIntWithDefault(
			"TAIWAN_STOCK_SYMBOLS_PER_LIVE_CHANNEL", 25),
		RequestTimeout: time.Duration(
			positiveIntWithDefault("TAIWAN_STOCK_REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
		RequestsPerMinute: positiveIntWithDefault("TAIWAN_STOCK_REQUESTS_PER_MINUTE", 55),
	}
}

// timeZoneWithDefault never returns nil, because a nil zone means the market never closes and a
// typo would silently make it so.
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

// timeOfDayWithDefault parses HH:MM into an offset from midnight.
func timeOfDayWithDefault(key string, defaultValue time.Duration) time.Duration {
	timeOfDay, parseError := time.Parse("15:04", os.Getenv(key))
	if parseError != nil {
		return defaultValue
	}

	return time.Duration(timeOfDay.Hour())*time.Hour + time.Duration(timeOfDay.Minute())*time.Minute
}

func positiveIntWithDefault(key string, defaultValue int) int {
	value, parseError := strconv.Atoi(os.Getenv(key))
	if parseError != nil || value <= 0 {
		return defaultValue
	}

	return value
}

// jobIntervalWithDefault treats zero or less as switching the job off, not as a mistake.
func jobIntervalWithDefault(key string, defaultValue int, unit time.Duration) time.Duration {
	value, parseError := strconv.Atoi(os.Getenv(key))
	if parseError != nil {
		return time.Duration(defaultValue) * unit
	}
	if value <= 0 {
		return 0
	}

	return time.Duration(value) * unit
}

func stringWithDefault(key string, defaultValue string) string {
	return cmp.Or(os.Getenv(key), defaultValue)
}

func boolWithDefault(key string, defaultValue bool) bool {
	value, parseError := strconv.ParseBool(os.Getenv(key))
	if parseError != nil {
		return defaultValue
	}

	return value
}

func commaSeparatedListWithDefault(key string, defaultValue []string) []string {
	entries := commaSeparatedList(key)
	if len(entries) == 0 {
		return defaultValue
	}

	return entries
}

// commaSeparatedList trims entries and drops empty ones; a missing variable is an empty list.
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

// HasAccountCredentials gates fetching the maintenance margin ladder.
func (contractIngestionConfig ContractIngestionConfig) HasAccountCredentials() bool {
	return contractIngestionConfig.AccountApiKey != "" && contractIngestionConfig.AccountApiSecret != ""
}
