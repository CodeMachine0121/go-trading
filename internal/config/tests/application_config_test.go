package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, "8080", applicationConfig.ServerPort)
	assert.Equal(t, 1000, applicationConfig.KCandleQueryMaxResults)
	assert.Equal(t, 40*time.Second, applicationConfig.IndicatorScriptTimeout)
	assert.Equal(t, 50000, applicationConfig.BacktestMaxCandleCount)
	assert.Equal(t, 90*time.Second, applicationConfig.BacktestTimeAllowance)
	assert.Equal(t, "localhost", applicationConfig.Database.Host)
	assert.Contains(t, applicationConfig.Database.DataSourceName(), "dbname=go_trading")
}

func TestLoadReadsTheEnvironment(t *testing.T) {
	testCases := []struct {
		name                   string
		queryMaxResults        string
		expectedQueryMaxResult int
	}{
		{name: "a usable count is taken as given", queryMaxResults: "250", expectedQueryMaxResult: 250},
		{name: "an unreadable count falls back", queryMaxResults: "many", expectedQueryMaxResult: 1000},
		{name: "zero falls back", queryMaxResults: "0", expectedQueryMaxResult: 1000},
		{name: "a negative count falls back", queryMaxResults: "-5", expectedQueryMaxResult: 1000},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "9090")
			t.Setenv("POSTGRES_HOST", "db.internal")
			t.Setenv("KCANDLE_QUERY_MAX_RESULTS", testCase.queryMaxResults)

			applicationConfig := config.Load()

			assert.Equal(t, "9090", applicationConfig.ServerPort)
			assert.Equal(t, "db.internal", applicationConfig.Database.Host)
			assert.Equal(t, testCase.expectedQueryMaxResult, applicationConfig.KCandleQueryMaxResults)
		})
	}
}

func TestLoadReadsTheIndicatorScriptAllowance(t *testing.T) {
	testCases := []struct {
		name            string
		allowanceValue  string
		expectedTimeout time.Duration
	}{
		{name: "a usable allowance is taken as given", allowanceValue: "5", expectedTimeout: 5 * time.Second},
		{name: "an unreadable allowance falls back", allowanceValue: "forever", expectedTimeout: 40 * time.Second},
		{name: "zero falls back", allowanceValue: "0", expectedTimeout: 40 * time.Second},
		{name: "a negative allowance falls back", allowanceValue: "-1", expectedTimeout: 40 * time.Second},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("INDICATOR_SCRIPT_TIMEOUT_SECONDS", testCase.allowanceValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedTimeout, applicationConfig.IndicatorScriptTimeout)
		})
	}
}

// The three live follow rules are settings rather than constants because each is a
// number the requirements name: a source that behaves differently is a value
// change, not a code change.
func TestLiveFollowRulesComeFromTheEnvironmentAndFallBackToTheStatedDefaults(t *testing.T) {
	t.Run("未設定時採用規則所寫的值", func(t *testing.T) {
		applicationConfig := config.Load()

		assert.Equal(t, 10*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.QuietTimeout)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.MaximumRetryDelay)
		assert.Equal(t, "wss://stream.binance.com:9443/ws",
			applicationConfig.LiveFollow.MarketDataStreamUrl)
	})

	t.Run("設定了就照設定的來", func(t *testing.T) {
		t.Setenv("LIVE_UPDATE_INTERVAL_CEILING_SECONDS", "3")
		t.Setenv("LIVE_FEED_QUIET_TIMEOUT_SECONDS", "45")
		t.Setenv("LIVE_FEED_MAX_RETRY_DELAY_SECONDS", "60")
		t.Setenv("MARKET_DATA_STREAM_URL", "ws://localhost:9000/ws")

		applicationConfig := config.Load()

		assert.Equal(t, 3*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 45*time.Second, applicationConfig.LiveFollow.QuietTimeout)
		assert.Equal(t, 60*time.Second, applicationConfig.LiveFollow.MaximumRetryDelay)
		assert.Equal(t, "ws://localhost:9000/ws", applicationConfig.LiveFollow.MarketDataStreamUrl)
	})

	t.Run("設成不成立的值就回到規則所寫的值", func(t *testing.T) {
		t.Setenv("LIVE_UPDATE_INTERVAL_CEILING_SECONDS", "0")
		t.Setenv("LIVE_FEED_QUIET_TIMEOUT_SECONDS", "不是數字")

		applicationConfig := config.Load()

		assert.Equal(t, 10*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.QuietTimeout)
	})
}

// The sealing key has no default and must not grow one. A default key is a key
// everybody running this code knows, and a secret locked with it is a secret in the
// open that looks locked.
func TestLoadLeavesTheSealingKeyEmptyWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Empty(t, applicationConfig.Secrets.SealKey)
}

func TestLoadReadsTheSealingKey(t *testing.T) {
	t.Setenv("SECRET_SEAL_KEY", "a-base64-key")

	applicationConfig := config.Load()

	assert.Equal(t, "a-base64-key", applicationConfig.Secrets.SealKey)
}

func TestLoadAppliesTelegramDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, "https://api.telegram.org", applicationConfig.Telegram.ApiBaseUrl)
	assert.Equal(t, 10*time.Second, applicationConfig.Telegram.RequestTimeout)
}

func TestLoadReadsHowLongItWillWaitForTelegram(t *testing.T) {
	testCases := []struct {
		name            string
		waitSeconds     string
		expectedTimeout time.Duration
	}{
		{name: "a usable wait is taken as given", waitSeconds: "3", expectedTimeout: 3 * time.Second},
		{name: "an unreadable wait falls back", waitSeconds: "soon", expectedTimeout: 10 * time.Second},
		// A wait of nothing is not a wait, it is a system that never reaches
		// Telegram at all — so it falls back rather than being honoured.
		{name: "zero falls back", waitSeconds: "0", expectedTimeout: 10 * time.Second},
		{name: "a negative wait falls back", waitSeconds: "-1", expectedTimeout: 10 * time.Second},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("TELEGRAM_API_BASE_URL", "http://localhost:9999")
			t.Setenv("TELEGRAM_REQUEST_TIMEOUT_SECONDS", testCase.waitSeconds)

			applicationConfig := config.Load()

			assert.Equal(t, "http://localhost:9999", applicationConfig.Telegram.ApiBaseUrl)
			assert.Equal(t, testCase.expectedTimeout, applicationConfig.Telegram.RequestTimeout)
		})
	}
}

func TestLoadGivesTheAssistantEnoughQueriesToFinishWhatItStarted(t *testing.T) {
	// The ceiling exists to stop one question costing without limit, but it also
	// decides whether the assistant can finish a thought. It now builds a trading
	// strategy, replays it, reads the report card, adjusts and replays again — about
	// five queries a turn — so a ceiling in single figures cuts it off right after it
	// has discovered the return is not good enough and before it can do anything
	// about it, which is the least useful place to stop.
	testCases := []struct {
		name          string
		queryLimit    string
		expectedLimit int
	}{
		{name: "nothing set leaves room for several turns", queryLimit: "", expectedLimit: 40},
		{name: "a usable ceiling is taken as given", queryLimit: "12", expectedLimit: 12},
		// A ceiling of nothing is not a ceiling, it is an assistant that may look at
		// nothing at all — so it falls back rather than being honoured.
		{name: "zero falls back", queryLimit: "0", expectedLimit: 40},
		{name: "a negative ceiling falls back", queryLimit: "-1", expectedLimit: 40},
		{name: "an unreadable ceiling falls back", queryLimit: "lots", expectedLimit: 40},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("ASSISTANT_QUERY_LIMIT", testCase.queryLimit)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedLimit, applicationConfig.Assistant.QueryLimit)
		})
	}
}

func TestLoadAppliesAccountActivationDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	// Both have a default, unlike the two keys, and the difference is that neither
	// is a key. A mailbox anybody can guess gives nothing away; refusing to start
	// for want of one would take the console down over a setting that guards nothing.
	assert.Equal(t,
		"james.afternoon.dev@gmail.com", applicationConfig.AccountActivation.RequestMailbox)
	assert.Equal(t, "go-trading 開通申請", applicationConfig.AccountActivation.SubjectPrefix)
}

func TestLoadReadsWhereToAskToBeLetIn(t *testing.T) {
	t.Setenv("ACCOUNT_ACTIVATION_REQUEST_MAILBOX", "gatekeeper@example.com")
	t.Setenv("ACCOUNT_ACTIVATION_SUBJECT_PREFIX", "console access request")

	applicationConfig := config.Load()

	assert.Equal(t,
		"gatekeeper@example.com", applicationConfig.AccountActivation.RequestMailbox)
	assert.Equal(t, "console access request", applicationConfig.AccountActivation.SubjectPrefix)
}

func TestLoadAppliesSignInLockoutDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, 3, applicationConfig.SignInLockout.FailureThreshold)
	assert.Equal(t, 7*24*time.Hour, applicationConfig.SignInLockout.LockoutDuration,
		"單位是天。寫成小時會讓一週的鎖安靜地變成七小時")
}

func TestLoadReadsHowTiredTheSignInDoorGets(t *testing.T) {
	t.Setenv("AUTH_SIGN_IN_FAILURE_THRESHOLD", "5")
	t.Setenv("AUTH_SIGN_IN_LOCKOUT_DAYS", "2")

	applicationConfig := config.Load()

	assert.Equal(t, 5, applicationConfig.SignInLockout.FailureThreshold)
	assert.Equal(t, 48*time.Hour, applicationConfig.SignInLockout.LockoutDuration)
}

func TestLoadRefusesToLetAnybodySwitchTheSignInLockOff(t *testing.T) {
	// A threshold of zero would shut every account on sight; a negative one, or a
	// duration of zero, would open the door to unlimited guessing. Both fall back,
	// so this lock has no "off" — which is the point, on an endpoint the whole
	// internet can reach.
	testCases := []struct {
		name      string
		threshold string
		days      string
	}{
		{name: "zero", threshold: "0", days: "0"},
		{name: "negative", threshold: "-1", days: "-7"},
		{name: "not a number", threshold: "off", days: "never"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("AUTH_SIGN_IN_FAILURE_THRESHOLD", testCase.threshold)
			t.Setenv("AUTH_SIGN_IN_LOCKOUT_DAYS", testCase.days)

			applicationConfig := config.Load()

			assert.Equal(t, 3, applicationConfig.SignInLockout.FailureThreshold)
			assert.Equal(t, 7*24*time.Hour, applicationConfig.SignInLockout.LockoutDuration)
		})
	}
}

func TestLoadReadsTheReplaySettings(t *testing.T) {
	t.Setenv("BACKTEST_MAX_CANDLE_COUNT", "100000")
	t.Setenv("BACKTEST_TIME_ALLOWANCE_SECONDS", "45")

	applicationConfig := config.Load()

	assert.Equal(t, 100000, applicationConfig.BacktestMaxCandleCount)
	assert.Equal(t, 45*time.Second, applicationConfig.BacktestTimeAllowance)
}
