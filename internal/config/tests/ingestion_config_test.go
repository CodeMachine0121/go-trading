package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesIngestionDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.True(t, applicationConfig.BackgroundJobsEnabled)
	assert.Equal(t, 25, applicationConfig.Ingestion.RoundCandleCount)
	assert.Equal(t, 24*time.Hour, applicationConfig.Ingestion.BackfillLookback)
	assert.Equal(t, 10*time.Second, applicationConfig.Ingestion.MarketDataRequestTimeout)
	assert.NotEmpty(t, applicationConfig.Ingestion.MarketDataBaseUrl)
}

func TestLoadReadsTheBackgroundJobSwitch(t *testing.T) {
	testCases := []struct {
		name            string
		switchValue     string
		expectedEnabled bool
	}{
		{name: "switched off", switchValue: "false", expectedEnabled: false},
		{name: "switched on", switchValue: "true", expectedEnabled: true},
		{name: "an answer that is neither falls back to on", switchValue: "maybe", expectedEnabled: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", testCase.switchValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedEnabled, applicationConfig.BackgroundJobsEnabled)
		})
	}
}

func TestLoadReadsTheIngestionAmounts(t *testing.T) {
	testCases := []struct {
		name                     string
		roundCandleCountValue    string
		backfillLookbackValue    string
		requestTimeoutValue      string
		expectedRoundCandleCount int
		expectedBackfillLookback time.Duration
		expectedRequestTimeout   time.Duration
	}{
		{
			name:                     "usable amounts are taken as given",
			roundCandleCountValue:    "8",
			backfillLookbackValue:    "6",
			requestTimeoutValue:      "3",
			expectedRoundCandleCount: 8,
			expectedBackfillLookback: 6 * time.Hour,
			expectedRequestTimeout:   3 * time.Second,
		},
		{
			name:                     "zero falls back",
			roundCandleCountValue:    "0",
			backfillLookbackValue:    "0",
			requestTimeoutValue:      "0",
			expectedRoundCandleCount: 25,
			expectedBackfillLookback: 24 * time.Hour,
			expectedRequestTimeout:   10 * time.Second,
		},
		{
			name:                     "unreadable or negative amounts fall back",
			roundCandleCountValue:    "-3",
			backfillLookbackValue:    "a day",
			requestTimeoutValue:      "-1",
			expectedRoundCandleCount: 25,
			expectedBackfillLookback: 24 * time.Hour,
			expectedRequestTimeout:   10 * time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("KCANDLE_INGESTION_ROUND_CANDLE_COUNT", testCase.roundCandleCountValue)
			t.Setenv("KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", testCase.backfillLookbackValue)
			t.Setenv("MARKET_DATA_REQUEST_TIMEOUT_SECONDS", testCase.requestTimeoutValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedRoundCandleCount, applicationConfig.Ingestion.RoundCandleCount)
			assert.Equal(t, testCase.expectedBackfillLookback, applicationConfig.Ingestion.BackfillLookback)
			assert.Equal(t, testCase.expectedRequestTimeout, applicationConfig.Ingestion.MarketDataRequestTimeout)
		})
	}
}

func TestLoadReadsTheMarketSourceAddress(t *testing.T) {
	t.Setenv("MARKET_DATA_BASE_URL", "http://127.0.0.1:9999/klines")

	applicationConfig := config.Load()

	assert.Equal(t, "http://127.0.0.1:9999/klines", applicationConfig.Ingestion.MarketDataBaseUrl)
}

func TestLoadAppliesTaiwanStockDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, 9*time.Hour, applicationConfig.TaiwanStock.SessionStart)
	assert.Equal(t, 13*time.Hour+30*time.Minute, applicationConfig.TaiwanStock.SessionEnd)
	assert.Equal(t, 25, applicationConfig.TaiwanStock.SymbolsPerLiveChannel)
	assert.Equal(t, "Asia/Taipei", applicationConfig.TaiwanStock.TimeZone.String())
	assert.NotEmpty(t, applicationConfig.TaiwanStock.IntradayCandlesUrl)
	assert.NotEmpty(t, applicationConfig.TaiwanStock.HistoricalCandlesUrl)
	assert.NotEmpty(t, applicationConfig.TaiwanStock.TickerUrl)
	assert.Equal(t, 5*time.Second, applicationConfig.TaiwanStock.LiveCandleInterval)
}

func TestLoadReadsTheTaiwanStockSessionAsATimeOfDay(t *testing.T) {
	t.Setenv("TAIWAN_STOCK_SESSION_START", "08:45")
	t.Setenv("TAIWAN_STOCK_SESSION_END", "14:30")

	applicationConfig := config.Load()

	assert.Equal(t, 8*time.Hour+45*time.Minute, applicationConfig.TaiwanStock.SessionStart)
	assert.Equal(t, 14*time.Hour+30*time.Minute, applicationConfig.TaiwanStock.SessionEnd)
}

func TestLoadFallsBackWhenTheSessionIsNotATimeOfDay(t *testing.T) {
	t.Setenv("TAIWAN_STOCK_SESSION_START", "quarter to nine")

	applicationConfig := config.Load()

	assert.Equal(t, 9*time.Hour, applicationConfig.TaiwanStock.SessionStart)
}

func TestLoadFallsBackWhenTheTimeZoneIsOneNobodyHasHeardOf(t *testing.T) {
	// A nil zone means the market never closes, so an unloadable zone must fall back rather than be
	// nil.
	t.Setenv("TAIWAN_STOCK_TIME_ZONE", "Asia/Taipeh")

	applicationConfig := config.Load()

	assert.Equal(t, "Asia/Taipei", applicationConfig.TaiwanStock.TimeZone.String())
	assert.NotNil(t, applicationConfig.MarketRules[vo.MarketTaiwanStock].TradingSession.Location)
}

func TestTheRecognisedMarketsCarryTheirOwnRules(t *testing.T) {
	applicationConfig := config.Load()

	// The round-the-clock market is entirely the zero value: no zone, no ceiling, no roster.
	assert.Nil(t, applicationConfig.MarketRules[vo.MarketCrypto].TradingSession.Location)
	assert.False(t, applicationConfig.MarketRules[vo.MarketCrypto].FollowsFixedRoster)
	assert.Equal(t, 0, applicationConfig.MarketRules[vo.MarketCrypto].SimultaneousChannelCeiling)
	assert.Equal(t, 0, applicationConfig.MarketRules[vo.MarketCrypto].SymbolsPerLiveChannel)

	// Taiwan follows an uncapped roster because the exchange sells no subscriptions.
	taiwanStockRules := applicationConfig.MarketRules[vo.MarketTaiwanStock]
	assert.True(t, taiwanStockRules.FollowsFixedRoster)
	assert.Equal(t, 0, taiwanStockRules.SimultaneousChannelCeiling)
	assert.Equal(t, 25, taiwanStockRules.SymbolsPerLiveChannel)
	assert.Equal(t, 9*time.Hour, taiwanStockRules.TradingSession.DailyStart)
	assert.Len(t, taiwanStockRules.TradingSession.Weekdays, 5)
}

func TestLoadBoundsHowFarBackOneHistorySyncMayReach(t *testing.T) {
	// The ceiling is a typo guard, not a cost bound.
	const defaultCeilingDays = 3650

	testCases := []struct {
		name         string
		lookbackDays string
		expectedDays int
	}{
		{
			name: "nothing set leaves room for a decade", lookbackDays: "",
			expectedDays: defaultCeilingDays,
		},
		{name: "a usable ceiling is taken as given", lookbackDays: "30", expectedDays: 30},
		{name: "zero falls back", lookbackDays: "0", expectedDays: defaultCeilingDays},
		{
			name: "a negative ceiling falls back", lookbackDays: "-1",
			expectedDays: defaultCeilingDays,
		},
		{
			name: "an unreadable ceiling falls back", lookbackDays: "a decade",
			expectedDays: defaultCeilingDays,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("KCANDLE_HISTORY_SYNC_MAX_LOOKBACK_DAYS", testCase.lookbackDays)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedDays,
				applicationConfig.Ingestion.HistorySyncMaxLookbackDays)
		})
	}
}
