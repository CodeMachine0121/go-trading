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
	Symbols          []string
	RoundCandleCount int
	BackfillLookback time.Duration
	// HistorySyncMaxLookbackDays is how far back one on-demand history sync may
	// reach. It is days rather than a duration because that is the unit the request
	// is made in, and the refusal has to quote it back.
	//
	// It is deliberately generous. What used to keep it small — the whole stretch
	// held in memory, one connection held open for the duration — is gone: the fetch
	// walks a chunk at a time, stores as it goes, and answers straight away with a run
	// to watch. So the ceiling is no longer about what the system can survive.
	//
	// What is left is a typo guard. Ten years is more history than any of these
	// sources will answer for, so a request inside it is a request somebody meant;
	// a request past it is a slipped digit, and the refusal says what the ceiling is.
	HistorySyncMaxLookbackDays int
	MarketDataBaseUrl          string
	// SymbolCatalogUrl is where a source is asked whether it lists a symbol at all.
	// It is separate from the candle address because they are separate questions, and
	// a source is free to answer them at different places.
	SymbolCatalogUrl         string
	MarketDataRequestTimeout time.Duration
	// MarketDataRequestsPerMinute is how fast this source is asked. It is a setting
	// rather than a constant because it is the venue's allowance, and a venue is free
	// to change what it allows without this system changing.
	MarketDataRequestsPerMinute int
}

// ContractIngestionConfig holds what reaching the perpetual contract venue runs on.
//
// Every value here has a spot counterpart and none of them is shared, which is the
// point. **The allowance in particular cannot be shared**: the two venues count their
// request budgets separately, so one pacer across both would spend half of each. And
// every contract candle takes two questions rather than one, so this side's request
// count is double the spot side's over the same stretch — a number that has to be
// settable on its own.
//
// The history ceiling is its own for a plainer reason: perpetual contracts have a
// much shorter past than the spot pairs beside them.
type ContractIngestionConfig struct {
	RoundCandleCount           int
	BackfillLookback           time.Duration
	HistorySyncMaxLookbackDays int
	// BaseUrl answers about the traded figures; the other three about the mark
	// price, the index price and the premium index. They are four addresses because
	// the venue keeps them apart, and one contract candle is assembled from all four.
	BaseUrl         string
	MarkPriceUrl    string
	IndexPriceUrl   string
	PremiumIndexUrl string
	// SymbolCatalogUrl is where a contract is confirmed to exist before it is
	// watched. This venue answers with its whole catalogue whatever it is asked, so
	// the address is the same question asked a different way from the spot one.
	SymbolCatalogUrl string
	// FundingInfoUrl lists the contracts whose funding rate settles on an interval of
	// their own. It is part of a contract's trading specification, and the venue
	// keeps it apart from the catalogue.
	FundingInfoUrl string
	// FundingRateUrl is where a contract's funding rate settlements are read. It
	// spends the same allowance as the candles.
	FundingRateUrl    string
	RequestTimeout    time.Duration
	RequestsPerMinute int
	// StatisticsBaseUrl is where the three answers a position statistic is assembled
	// from live. The venue counts these apart from everything else it serves, which
	// is why they have an allowance of their own.
	StatisticsBaseUrl           string
	StatisticsRequestsPerMinute int
	// PositionStatisticArchiveBaseUrl is the venue's history archive of the same
	// statistics, one file per contract per day and years deep, where the live
	// addresses above keep thirty days. A different host, so an allowance of its own.
	PositionStatisticArchiveBaseUrl           string
	PositionStatisticArchiveRequestsPerMinute int
	// The three rounds beside the candles, each at the pace its own data changes: a
	// funding rate settles a few times a day, a position statistic is taken every
	// five minutes, and a trading specification barely changes at all. Zero switches
	// that round off.
	FundingRateIngestionInterval        time.Duration
	PositionStatisticIngestionInterval  time.Duration
	TradingSpecificationRefreshInterval time.Duration
	// AccountApiKey and AccountApiSecret prove to the venue which account is asking.
	// Only the maintenance margin ladder needs them, and only read access: nothing
	// here trades or moves funds. They have no default and must not have one — a key
	// everyone running this code shares is a key anyone can use. Left empty, the
	// ladder is simply not fetched and everything else runs as before.
	AccountApiKey    string
	AccountApiSecret string
	// MaintenanceMarginTierUrl is where the full ladder is asked for, as the account.
	MaintenanceMarginTierUrl             string
	MaintenanceMarginTierRefreshInterval time.Duration
}

// LiveFollowConfig holds the three rules a live follow behaves by. All three carry
// a number the requirements name, so they are settings rather than constants: a
// source that behaves differently is a value change, not a code change.
type LiveFollowConfig struct {
	UpdateIntervalCeiling time.Duration
	QuietTimeout          time.Duration
	MaximumRetryDelay     time.Duration
	MarketDataStreamUrl   string
	// ContractMarketDataStreamUrl is where the perpetual contract venue's live candles
	// are followed. It is the one live setting the contract line has of its own: the
	// three timing rules describe a person watching a chart and a line staying up,
	// neither of which depends on which market it is.
	ContractMarketDataStreamUrl string
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
	// LiveCandleInterval is how often a live follow asks this venue for today's
	// candles. Live and history come from the same place, so there is no second
	// address and no second allowance — the two share RequestsPerMinute below.
	//
	// One request buys one symbol, so what a follow actually manages is this or the
	// allowance, whichever is slower; the proxy works that out rather than letting the
	// two quietly disagree.
	LiveCandleInterval time.Duration
	// TimeZone is the zone this market states its hours in. "Nine o'clock" is a fact
	// about Taipei, and the moment it names universally is not the same one all year
	// in markets that shift with daylight saving.
	TimeZone *time.Location
	// SessionStart and SessionEnd are how far into its local day trading begins and
	// ends.
	SessionStart time.Duration
	SessionEnd   time.Duration
	// SymbolsPerLiveChannel is how many trading symbols one round of questions
	// covers. Every symbol is asked for under both of the boards it might be listed
	// on, so the request carries twice this many entries — which is why it is well
	// under what the exchange answers in one go rather than equal to it.
	//
	// It is no longer a subscription limit: the exchange does not sell them. A roster
	// longer than this is asked in several rounds, and nothing is left unfollowed.
	// The old pair of numbers were the market data plan's shape, and that plan no
	// longer supplies the live feed at all.
	SymbolsPerLiveChannel int
	RequestTimeout        time.Duration
	// RequestsPerMinute is how fast this source is asked. It matters more here than
	// it does for the crypto venue: this one answers about one local day per request,
	// so a stretch of years is thousands of them in a row.
	RequestsPerMinute int
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
	//
	// It is forty rather than a handful because the assistant now closes a loop
	// inside one answer: it assembles a set of rules, replays it, reads the report
	// card, changes something and replays again. One turn of that costs about five
	// queries, so a ceiling of eight cuts it off mid-thought — right after it has
	// discovered the return is not good enough and before it can do anything about
	// it, which is the least useful place to stop.
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

// AccountActivationConfig holds where somebody asks to be let in, and how that letter
// announces itself.
//
// Unlike the two keys above and below, both halves have a default, and the difference
// is that neither is a key. A mailbox anybody can guess gives nothing away; refusing
// to start for want of one would take the whole console down over a setting that
// guards nothing.
type AccountActivationConfig struct {
	// RequestMailbox is the address a person waiting to be let in writes to.
	RequestMailbox string
	// SubjectPrefix opens that letter's subject; the applicant's own address closes
	// it, so that the inbox can be read as a list of who is asking.
	SubjectPrefix string
}

// SignInLockoutConfig holds how tired the sign-in door is allowed to get.
//
// Both halves have defaults and neither is a key: a console that configured nothing
// still shuts an account after three wrong passwords, because the setting that
// guards the door should not be one somebody has to remember to switch on.
//
// They are settings at all so that a test can watch a week-long lock end without
// waiting a week.
type SignInLockoutConfig struct {
	// FailureThreshold is how many consecutive wrong passwords shut an account.
	FailureThreshold int
	// LockoutDuration is how long it stays shut.
	LockoutDuration time.Duration
}

// SecretsConfig holds what this system locks away secrets with.
//
// The key has no default and cannot have one, for the same reason the access token
// signing key cannot: a default key is a key everybody running this code knows, and
// a secret locked with a key everybody knows is a secret in the open that looks
// locked. Leaving it unset means no secret can be stored — which is the correct
// thing for a system with no key to do, and is why the paths that need it say so out
// loud instead of quietly working in a way that guards nothing.
type SecretsConfig struct {
	// SealKey is thirty-two bytes, written as base64. Anything else is treated as
	// no key at all rather than padded or truncated into one.
	SealKey string
}

// TelegramConfig holds what reaching Telegram runs on.
//
// Both are settings rather than constants because both are facts about somebody
// else's service: where it answers, and how long this system is willing to wait for
// it. A test points the first at a stand-in; an impatient deployment shortens the
// second.
type TelegramConfig struct {
	ApiBaseUrl     string
	RequestTimeout time.Duration
}

// StrategyBotConfig is how the standing bots are run.
//
// All three are settings rather than constants because all three are about this
// deployment's appetite rather than about what a bot means. The limits that *are*
// about what a bot means — how deep a condition may nest, how many sources it may
// have, how many bots one person may run at once — are written in the domain, where
// changing one is a change to the rules and not to a machine.
type StrategyBotConfig struct {
	// ScanInterval is how often the due bots are looked for. A minute matches the
	// shortest trigger interval a bot may have, so nothing waits longer for its
	// round than it asked to.
	ScanInterval time.Duration
	// MaxConcurrentRounds caps both how many bots one scan pulls out of the store
	// and how many run side by side. One number rather than two: a round that is
	// read and then not run is a read that bought nothing.
	MaxConcurrentRounds int
	// RoundTimeout is how long one bot's round may take before it is abandoned. A
	// round that outlives its own trigger interval has stopped being about now.
	RoundTimeout time.Duration
}

// ApplicationConfig holds every setting the binaries read from the environment.
type ApplicationConfig struct {
	ServerPort             string
	CorsAllowedOrigins     []string
	KCandleQueryMaxResults int
	IndicatorScriptTimeout time.Duration
	// IndicatorScriptMemoryLimitBytes is the most memory one script compartment may
	// take. It has to sit well below the memory the service itself is given: several
	// compartments can be running at once, and the point of the cap is that none of
	// them can take the service down.
	IndicatorScriptMemoryLimitBytes int64
	// BacktestMaxCandleCount is how many buckets one replay may walk. It is the
	// replay's own, no longer the single-query ceiling: a replay is one question that
	// reads the market once, but it walks far more than any one query hands back.
	BacktestMaxCandleCount int
	// BacktestTimeAllowance is how long one whole replay may take, reading included. It
	// sits below the hundred seconds a reverse proxy commonly waits, so that a replay
	// says it ran out of time rather than being cut off without a word.
	BacktestTimeAllowance time.Duration
	BackgroundJobsEnabled bool
	Ingestion             IngestionConfig
	ContractIngestion     ContractIngestionConfig
	LiveFollow            LiveFollowConfig
	TaiwanStock           TaiwanStockConfig
	// MarketRules is how every market the system recognises behaves. Recognising one
	// more market is one more entry here and two more sources wired to it; nothing
	// inside the system branches on which market it is looking at.
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
		IndicatorScriptMemoryLimitBytes: int64(
			positiveIntWithDefault("INDICATOR_SCRIPT_MEMORY_LIMIT_MEGABYTES", 512)) << 20,
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
			// Comfortably inside what this venue allows a candle request. The
			// headroom is deliberate: the allowance is shared with everything else
			// this system asks the venue, and being throttled costs more than
			// being slower.
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
			// The venue allows a thousand of these every five minutes; this stays a
			// little under that, so a thirty-day catch-up and the five-minute round
			// never meet the ceiling together.
			StatisticsRequestsPerMinute: positiveIntWithDefault(
				"CONTRACT_MARKET_DATA_STATISTICS_REQUESTS_PER_MINUTE", 180),
			PositionStatisticArchiveBaseUrl: stringWithDefault(
				"CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_BASE_URL",
				"https://data.binance.vision/data/futures/um/daily/metrics"),
			// The archive is a static file host that publishes no allowance. Two a
			// second walks four years of days in under a quarter of an hour, which is
			// already far quicker than the candles of the same stretch.
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
			// Half the spot allowance by default, because each candle here costs two
			// requests rather than one — so the same number of candles a minute is
			// reached from half the number written down.
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
			// Low effort is the right default for a conversation: the thinking that
			// higher effort buys pays off on hard problems, and "how has BTCUSDT
			// moved" is not one. The model itself is left at the capable one, because
			// picking the wrong capability costs more round trips than thinking
			// harder ever saves.
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
		AccountActivation: AccountActivationConfig{
			RequestMailbox: stringWithDefault(
				"ACCOUNT_ACTIVATION_REQUEST_MAILBOX", "james.afternoon.dev@gmail.com"),
			SubjectPrefix: stringWithDefault(
				"ACCOUNT_ACTIVATION_SUBJECT_PREFIX", "go-trading 開通申請"),
		},
		SignInLockout: SignInLockoutConfig{
			FailureThreshold: positiveIntWithDefault("AUTH_SIGN_IN_FAILURE_THRESHOLD", 3),
			// A week, and the length is the point. A lock measured in minutes only
			// slows a machine that does not get bored, while costing the account
			// holder the same shut door; with no self-service way back in and a
			// handful of people using this console, the cost of erring long is a
			// conversation and one edited row.
			LockoutDuration: time.Duration(
				positiveIntWithDefault("AUTH_SIGN_IN_LOCKOUT_DAYS", 7)) * 24 * time.Hour,
		},
		Secrets: SecretsConfig{
			SealKey: stringWithDefault("SECRET_SEAL_KEY", ""),
		},
		Telegram: TelegramConfig{
			ApiBaseUrl: stringWithDefault("TELEGRAM_API_BASE_URL", "https://api.telegram.org"),
			// Ten seconds is long enough for a message that is going to arrive and
			// short enough that somebody pressing a button to find out whether the
			// route works is not left wondering.
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
			// Taiwan is followed from a roster: every watched stock, whether or not
			// anybody is looking. Crypto is followed by whoever opens a chart, which
			// is the zero value and therefore says itself.
			//
			// Nothing caps how many may be followed — the exchange does not sell
			// subscriptions — so the ceiling is left at the zero value that means "no
			// ceiling". The rule is kept rather than deleted because a capped market
			// is a thing venues really do sell, and it costs one unread field.
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

// jobIntervalWithDefault reads how often a background job runs, in the unit given. A
// missing or unreadable variable falls back to the default; zero or less is a switch,
// not a mistake — it turns that one job off.
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

// HasAccountCredentials says whether both halves of the account's key are set. The
// maintenance margin ladder is only fetched when they are.
func (contractIngestionConfig ContractIngestionConfig) HasAccountCredentials() bool {
	return contractIngestionConfig.AccountApiKey != "" && contractIngestionConfig.AccountApiSecret != ""
}
