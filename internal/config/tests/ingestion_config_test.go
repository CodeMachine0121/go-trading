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
	assert.Equal(t, 1, applicationConfig.TaiwanStock.SimultaneousChannelCeiling)
	assert.Equal(t, 5, applicationConfig.TaiwanStock.SymbolsPerLiveChannel)
	assert.Equal(t, "Asia/Taipei", applicationConfig.TaiwanStock.TimeZone.String())
	assert.NotEmpty(t, applicationConfig.TaiwanStock.IntradayCandlesUrl)
	assert.NotEmpty(t, applicationConfig.TaiwanStock.HistoricalCandlesUrl)
	assert.NotEmpty(t, applicationConfig.TaiwanStock.TickerUrl)
	assert.NotEmpty(t, applicationConfig.TaiwanStock.StreamUrl)
}

func TestLoadReadsTheTaiwanStockSessionAsATimeOfDay(t *testing.T) {
	t.Setenv("TAIWAN_STOCK_SESSION_START", "08:45")
	t.Setenv("TAIWAN_STOCK_SESSION_END", "14:30")

	applicationConfig := config.Load()

	assert.Equal(t, 8*time.Hour+45*time.Minute, applicationConfig.TaiwanStock.SessionStart)
	assert.Equal(t, 14*time.Hour+30*time.Minute, applicationConfig.TaiwanStock.SessionEnd)
}

func TestLoadFallsBackWhenTheSessionIsNotATimeOfDay(t *testing.T) {
	// A market left with no hours at all would read as one that never closes, which
	// is the opposite of what a typo here was trying to say.
	t.Setenv("TAIWAN_STOCK_SESSION_START", "quarter to nine")

	applicationConfig := config.Load()

	assert.Equal(t, 9*time.Hour, applicationConfig.TaiwanStock.SessionStart)
}

func TestLoadFallsBackWhenTheTimeZoneIsOneNobodyHasHeardOf(t *testing.T) {
	// A zone that cannot be loaded must not leave the market with none: no zone is how
	// a market says it never closes, so a typo would quietly turn a market that shuts
	// every evening into one that never does.
	t.Setenv("TAIWAN_STOCK_TIME_ZONE", "Asia/Taipeh")

	applicationConfig := config.Load()

	assert.Equal(t, "Asia/Taipei", applicationConfig.TaiwanStock.TimeZone.String())
	assert.NotNil(t, applicationConfig.MarketRules[vo.MarketTaiwanStock].TradingSession.Location)
}

func TestTheRecognisedMarketsCarryTheirOwnRules(t *testing.T) {
	applicationConfig := config.Load()

	// The round-the-clock market says it never closes by having no zone to state hours
	// in, no ceiling on how many channels may be open, and no need to say how many
	// symbols one carries — every unfilled number is the zero value.
	assert.Nil(t, applicationConfig.MarketRules[vo.MarketCrypto].TradingSession.Location)
	assert.Equal(t, 0, applicationConfig.MarketRules[vo.MarketCrypto].SimultaneousChannelCeiling)
	assert.Equal(t, 0, applicationConfig.MarketRules[vo.MarketCrypto].SymbolsPerLiveChannel)

	taiwanStockRules := applicationConfig.MarketRules[vo.MarketTaiwanStock]
	assert.Equal(t, 1, taiwanStockRules.SimultaneousChannelCeiling)
	assert.Equal(t, 5, taiwanStockRules.SymbolsPerLiveChannel)
	assert.Equal(t, []vo.TradingStretchVo{{
		StartOffset: 9 * time.Hour,
		EndOffset:   13*time.Hour + 30*time.Minute,
	}}, taiwanStockRules.TradingSession.Stretches)
	assert.Len(t, taiwanStockRules.TradingSession.Weekdays, 5)
}

func TestTheFuturesMarketTradesTwoBoardsAndTheSecondRunsPastMidnight(t *testing.T) {
	applicationConfig := config.Load()

	taiwanFuturesRules := applicationConfig.MarketRules[vo.MarketTaiwanFutures]

	// The evening board's end is written on the clock as five in the morning, and read
	// as twenty-nine hours into the day it started — which is what "past midnight"
	// means once it has to be compared with anything.
	assert.Equal(t, []vo.TradingStretchVo{
		{
			StartOffset: 8*time.Hour + 45*time.Minute,
			EndOffset:   13*time.Hour + 45*time.Minute,
		},
		{
			StartOffset:              15 * time.Hour,
			EndOffset:                29 * time.Hour,
			BelongsToNextBusinessDay: true,
		},
	}, taiwanFuturesRules.TradingSession.Stretches)
	assert.Len(t, taiwanFuturesRules.TradingSession.Weekdays, 5)
	assert.Equal(t, 1, taiwanFuturesRules.SimultaneousChannelCeiling)
	assert.Equal(t, 5, taiwanFuturesRules.SymbolsPerLiveChannel)
}

func TestAnEveningBoardEndingLaterOnTheClockIsTakenAsGiven(t *testing.T) {
	// A venue whose second board shuts before midnight needs no carrying over, and
	// reading one in would put its close a day out.
	t.Setenv("TAIWAN_FUTURES_EVENING_BOARD_START", "15:00")
	t.Setenv("TAIWAN_FUTURES_EVENING_BOARD_END", "20:00")

	applicationConfig := config.Load()

	eveningBoard := applicationConfig.MarketRules[vo.MarketTaiwanFutures].
		TradingSession.Stretches[1]
	assert.Equal(t, 20*time.Hour, eveningBoard.EndOffset)
}
