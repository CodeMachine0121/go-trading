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

func TestLiveFollowRulesComeFromTheEnvironmentAndFallBackToTheStatedDefaults(t *testing.T) {
	t.Run("未設定時採用規則所寫的值", func(t *testing.T) {
		applicationConfig := config.Load()

		assert.Equal(t, 10*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.QuietTimeout)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.MaximumRetryDelay)
		assert.Equal(t, "wss://stream.binance.com:9443/ws",
			applicationConfig.LiveFollow.MarketDataStreamUrl)
		assert.Equal(t, "wss://fstream.binance.com/ws",
			applicationConfig.LiveFollow.ContractMarketDataStreamUrl)
	})

	t.Run("設定了就照設定的來", func(t *testing.T) {
		t.Setenv("LIVE_UPDATE_INTERVAL_CEILING_SECONDS", "3")
		t.Setenv("LIVE_FEED_QUIET_TIMEOUT_SECONDS", "45")
		t.Setenv("LIVE_FEED_MAX_RETRY_DELAY_SECONDS", "60")
		t.Setenv("MARKET_DATA_STREAM_URL", "ws://localhost:9000/ws")
		t.Setenv("CONTRACT_MARKET_DATA_STREAM_URL", "ws://localhost:9001/ws")

		applicationConfig := config.Load()

		assert.Equal(t, 3*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 45*time.Second, applicationConfig.LiveFollow.QuietTimeout)
		assert.Equal(t, 60*time.Second, applicationConfig.LiveFollow.MaximumRetryDelay)
		assert.Equal(t, "ws://localhost:9000/ws", applicationConfig.LiveFollow.MarketDataStreamUrl)
		assert.Equal(t, "ws://localhost:9001/ws", applicationConfig.LiveFollow.ContractMarketDataStreamUrl)
	})

	t.Run("設成不成立的值就回到規則所寫的值", func(t *testing.T) {
		t.Setenv("LIVE_UPDATE_INTERVAL_CEILING_SECONDS", "0")
		t.Setenv("LIVE_FEED_QUIET_TIMEOUT_SECONDS", "不是數字")

		applicationConfig := config.Load()

		assert.Equal(t, 10*time.Second, applicationConfig.LiveFollow.UpdateIntervalCeiling)
		assert.Equal(t, 30*time.Second, applicationConfig.LiveFollow.QuietTimeout)
	})
}

// A default sealing key would be known to everyone running this code, so none must exist.
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
		// A zero timeout would never reach Telegram, so it falls back to the default.
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
	// The ceiling must allow build-replay-adjust loops of about five queries per turn.
	testCases := []struct {
		name          string
		queryLimit    string
		expectedLimit int
	}{
		{name: "nothing set leaves room for several turns", queryLimit: "", expectedLimit: 40},
		{name: "a usable ceiling is taken as given", queryLimit: "12", expectedLimit: 12},
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

	// Unlike the keys these have defaults, since neither value is a secret.
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
	// Zero or negative values fall back, so the lockout can never be switched off on an
	// internet-facing endpoint.
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

func TestLoadReadsTheIndicatorScriptMemoryLimit(t *testing.T) {
	testCases := []struct {
		name          string
		limitValue    string
		expectedBytes int64
	}{
		{name: "a usable limit is taken as given, in megabytes", limitValue: "256", expectedBytes: 256 << 20},
		{name: "an unreadable limit falls back", limitValue: "plenty", expectedBytes: 512 << 20},
		{name: "zero falls back rather than lifting the cap", limitValue: "0", expectedBytes: 512 << 20},
		{name: "a negative limit falls back", limitValue: "-1", expectedBytes: 512 << 20},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("INDICATOR_SCRIPT_MEMORY_LIMIT_MEGABYTES", testCase.limitValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedBytes, applicationConfig.IndicatorScriptMemoryLimitBytes)
		})
	}
}

func TestLoadReadsTheIndicatorScriptCompartmentCap(t *testing.T) {
	testCases := []struct {
		name          string
		capValue      string
		expectedCount int
	}{
		{name: "a usable cap is taken as given", capValue: "3", expectedCount: 3},
		{name: "an unreadable cap falls back", capValue: "many", expectedCount: 6},
		{name: "zero falls back rather than stopping every script", capValue: "0", expectedCount: 6},
		{name: "a negative cap falls back", capValue: "-2", expectedCount: 6},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("INDICATOR_SCRIPT_MAX_CONCURRENT_COMPARTMENTS", testCase.capValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedCount, applicationConfig.IndicatorScriptMaxConcurrentCompartments)
		})
	}
}

func TestLoadReadsThePublicAddressesForConnectorAuthorization(t *testing.T) {
	testCases := []struct {
		name                    string
		publicBaseUrl           string
		frontendBaseUrl         string
		expectedPublicBaseUrl   string
		expectedFrontendBaseUrl string
	}{
		{name: "unset falls back to local addresses", expectedPublicBaseUrl: "http://localhost:8080", expectedFrontendBaseUrl: "http://localhost:3000"},
		{
			name:          "a trailing slash is trimmed",
			publicBaseUrl: "https://trading-api.example.com/", frontendBaseUrl: "https://web.example.com/",
			expectedPublicBaseUrl: "https://trading-api.example.com", expectedFrontendBaseUrl: "https://web.example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("PUBLIC_BASE_URL", testCase.publicBaseUrl)
			t.Setenv("FRONTEND_BASE_URL", testCase.frontendBaseUrl)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedPublicBaseUrl, applicationConfig.ConnectorAuthorization.PublicBaseUrl)
			assert.Equal(t, testCase.expectedFrontendBaseUrl, applicationConfig.ConnectorAuthorization.FrontendBaseUrl)
		})
	}
}
