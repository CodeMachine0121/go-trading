package main

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestEachVenueIsPacedOnItsOwnAllowance holds the one wiring fact nothing else can:
// the contract venue is paced separately from the spot one.
//
// The two venues count their request budgets apart, so a pacer shared between them
// would spend half of each — and the failure is silent, because a shared pacer works
// perfectly until the day a long fetch is throttled halfway through.
func TestEachVenueIsPacedOnItsOwnAllowance(t *testing.T) {
	pacers := newVenuePacers(config.Load())

	assert.NotSame(t, pacers.crypto.Limiter(), pacers.cryptoContract.Limiter(),
		"合約與現貨必須各有一份節奏，共用一份等於各只用到一半的額度")
	assert.NotSame(t, pacers.crypto.Limiter(), pacers.taiwanStock.Limiter())
	assert.NotSame(t, pacers.cryptoContract.Limiter(), pacers.taiwanStock.Limiter())
	// The contract venue counts its position statistics apart from its candles.
	assert.NotSame(t, pacers.cryptoContract.Limiter(), pacers.cryptoContractStatistics.Limiter(),
		"持倉統計與合約 K 線各有一份額度")
	assert.NotSame(t, pacers.crypto.Limiter(), pacers.cryptoContractStatistics.Limiter())
	// The archive of those statistics is another host again, counting nothing
	// against either of the contract venue's allowances.
	for _, otherLimiter := range []struct {
		name    string
		pacerOf func(venuePacers) *rate.Limiter
	}{
		{"合約 K 線", func(pacers venuePacers) *rate.Limiter { return pacers.cryptoContract.Limiter() }},
		{"持倉統計", func(pacers venuePacers) *rate.Limiter { return pacers.cryptoContractStatistics.Limiter() }},
		{"現貨", func(pacers venuePacers) *rate.Limiter { return pacers.crypto.Limiter() }},
	} {
		assert.NotSame(t, otherLimiter.pacerOf(pacers), pacers.cryptoContractArchive.Limiter(),
			"持倉統計歷史資料庫與%s各有一份節奏", otherLimiter.name)
	}
}

// TestEachVenuesPaceComesFromItsOwnSetting checks the other half: separate pacers
// built from the same number would be two pacers that only look independent.
func TestEachVenuesPaceComesFromItsOwnSetting(t *testing.T) {
	t.Setenv("MARKET_DATA_REQUESTS_PER_MINUTE", "600")
	t.Setenv("CONTRACT_MARKET_DATA_REQUESTS_PER_MINUTE", "111")
	t.Setenv("CONTRACT_MARKET_DATA_STATISTICS_REQUESTS_PER_MINUTE", "77")
	t.Setenv("CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_REQUESTS_PER_MINUTE", "33")

	applicationConfig := config.Load()

	require.Equal(t, 600, applicationConfig.Ingestion.MarketDataRequestsPerMinute)
	assert.Equal(t, 111, applicationConfig.ContractIngestion.RequestsPerMinute)
	assert.Equal(t, 77, applicationConfig.ContractIngestion.StatisticsRequestsPerMinute)
	assert.Equal(t, 33, applicationConfig.ContractIngestion.PositionStatisticArchiveRequestsPerMinute)
}
