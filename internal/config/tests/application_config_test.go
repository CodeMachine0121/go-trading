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
