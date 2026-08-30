package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesIngestionDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.True(t, applicationConfig.BackgroundJobsEnabled)
	assert.Empty(t, applicationConfig.Ingestion.Symbols)
	assert.Equal(t, 5, applicationConfig.Ingestion.RoundCandleCount)
	assert.Equal(t, 24*time.Hour, applicationConfig.Ingestion.BackfillLookback)
	assert.Equal(t, 10*time.Second, applicationConfig.Ingestion.MarketDataRequestTimeout)
	assert.NotEmpty(t, applicationConfig.Ingestion.MarketDataBaseUrl)
}

func TestLoadReadsTheWatchlist(t *testing.T) {
	testCases := []struct {
		name            string
		watchlistValue  string
		expectedSymbols []string
	}{
		{
			name:            "one symbol per comma",
			watchlistValue:  "BTCUSDT,ETHUSDT",
			expectedSymbols: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:            "spaces around a symbol are ignored",
			watchlistValue:  " BTCUSDT , ETHUSDT ",
			expectedSymbols: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:            "empty entries are dropped",
			watchlistValue:  "BTCUSDT,,ETHUSDT,",
			expectedSymbols: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:            "a watchlist of nothing but separators is empty",
			watchlistValue:  ",,",
			expectedSymbols: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("KCANDLE_INGESTION_SYMBOLS", testCase.watchlistValue)

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedSymbols, applicationConfig.Ingestion.Symbols)
		})
	}
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
			expectedRoundCandleCount: 5,
			expectedBackfillLookback: 24 * time.Hour,
			expectedRequestTimeout:   10 * time.Second,
		},
		{
			name:                     "unreadable or negative amounts fall back",
			roundCandleCountValue:    "-3",
			backfillLookbackValue:    "a day",
			requestTimeoutValue:      "-1",
			expectedRoundCandleCount: 5,
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
