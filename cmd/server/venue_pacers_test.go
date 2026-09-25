package main

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestEachVenueIsPacedOnItsOwnAllowance guards that venues with separate request budgets never share
// a pacer, which would silently halve each budget.
func TestEachVenueIsPacedOnItsOwnAllowance(t *testing.T) {
	pacers := newVenuePacers(config.Load())

	assert.NotSame(t, pacers.crypto.Limiter(), pacers.cryptoContract.Limiter(),
		"合約與現貨必須各有一份節奏，共用一份等於各只用到一半的額度")
	assert.NotSame(t, pacers.crypto.Limiter(), pacers.taiwanStock.Limiter())
	assert.NotSame(t, pacers.cryptoContract.Limiter(), pacers.taiwanStock.Limiter())
	assert.NotSame(t, pacers.cryptoContract.Limiter(), pacers.cryptoContractStatistics.Limiter(),
		"持倉統計與合約 K 線各有一份額度")
	assert.NotSame(t, pacers.crypto.Limiter(), pacers.cryptoContractStatistics.Limiter())
	// The statistics archive is a separate host with its own allowance.
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
