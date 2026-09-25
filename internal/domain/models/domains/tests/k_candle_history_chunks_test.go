package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var historyTaipei = time.FixedZone("Asia/Taipei", 8*60*60)

func historyMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   historyTaipei,
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
		},
	})
}

func historyIngestionDomainAt(t *testing.T, currentTime time.Time) domains.KCandleIngestionDomain {
	t.Helper()

	ingestionDomain, buildError := domains.NewKCandleIngestionDomain(currentTime, 5, 24*time.Hour)
	require.NoError(t, buildError)

	return ingestionDomain
}

func TestHistoryChunksAreOneDayEachOldestFirst(t *testing.T) {
	// Chunks are one day each, oldest first, so a run that dies midway leaves the gap at the recent end where the ordinary backfill closes it.
	ingestionDomain := historyIngestionDomainAt(t, time.Date(2026, 9, 17, 10, 30, 30, 0, time.UTC))

	chunks := ingestionDomain.HistoryChunks(
		"BTCUSDT", vo.MarketCrypto, 3*24*time.Hour)

	require.Len(t, chunks, 4)
	assert.Equal(t, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), chunks[0].StartTime)
	assert.Equal(t, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), chunks[1].StartTime)
	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), chunks[2].StartTime)
	assert.Equal(t, time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), chunks[3].StartTime)

	// Each ends on the last open time of its own day…
	assert.Equal(t, time.Date(2026, 9, 14, 23, 59, 0, 0, time.UTC), chunks[0].EndTime)
	// …except the last, which stops at the last minute that has actually closed.
	assert.Equal(t, time.Date(2026, 9, 17, 10, 29, 0, 0, time.UTC), chunks[3].EndTime)
}

func TestHistoryChunksCoverExactlyTheStretchAndNothingMore(t *testing.T) {
	// Chunks must abut with no gap (a minute never fetched) and no overlap (a request paid twice).
	ingestionDomain := historyIngestionDomainAt(t, time.Date(2026, 9, 17, 10, 30, 30, 0, time.UTC))

	chunks := ingestionDomain.HistoryChunks("BTCUSDT", vo.MarketCrypto, 5*24*time.Hour)

	require.NotEmpty(t, chunks)
	for index := 1; index < len(chunks); index++ {
		assert.Equal(t, chunks[index-1].EndTime.Add(time.Minute), chunks[index].StartTime,
			"第 %d 段要正好接在前一段後面", index)
	}

	whole := ingestionDomain.HistoryWindow("BTCUSDT", vo.MarketCrypto, 5*24*time.Hour)
	assert.Equal(t, whole.StartTime, chunks[0].StartTime)
	assert.Equal(t, whole.EndTime, chunks[len(chunks)-1].EndTime)
}

func TestHistoryChunksCarryTheSymbolAndMarketEachNeeds(t *testing.T) {
	ingestionDomain := historyIngestionDomainAt(t, time.Date(2026, 9, 17, 10, 30, 30, 0, time.UTC))

	chunks := ingestionDomain.HistoryChunks("2330", vo.MarketTaiwanStock, 2*24*time.Hour)

	require.NotEmpty(t, chunks)
	for _, chunk := range chunks {
		assert.Equal(t, "2330", chunk.Symbol)
		assert.Equal(t, vo.MarketTaiwanStock, chunk.Market)
	}
}

func TestTradingKCandleCountBetweenCountsBothEnds(t *testing.T) {
	// The count is inclusive so comparing it with storage can skip a day that is already complete.
	testCases := []struct {
		name          string
		market        vo.MarketVo
		startTime     time.Time
		endTime       time.Time
		expectedCount int
	}{
		{
			// TradingBucketCountBetween deliberately answers 1439 here; this is the separate inclusive count.
			name: "a whole day of a market that never closes", market: vo.MarketCrypto,
			startTime:     time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
			endTime:       time.Date(2026, 9, 14, 23, 59, 0, 0, time.UTC),
			expectedCount: 1440,
		},
		{
			name: "one minute", market: vo.MarketCrypto,
			startTime:     time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
			endTime:       time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
			expectedCount: 1,
		},
		{
			// 09:00 to 13:29 inclusive is four and a half hours of open times.
			name: "a whole session of a market that closes", market: vo.MarketTaiwanStock,
			startTime:     time.Date(2026, 9, 14, 0, 0, 0, 0, historyTaipei),
			endTime:       time.Date(2026, 9, 14, 23, 59, 0, 0, historyTaipei),
			expectedCount: 270,
		},
		{
			name: "a day that market is shut on", market: vo.MarketTaiwanStock,
			startTime:     time.Date(2026, 9, 12, 0, 0, 0, 0, historyTaipei),
			endTime:       time.Date(2026, 9, 12, 23, 59, 0, 0, historyTaipei),
			expectedCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			marketDomain := historyMarketCatalog().MarketOf(string(testCase.market))

			assert.Equal(t, testCase.expectedCount,
				marketDomain.TradingKCandleCountBetween(testCase.startTime, testCase.endTime))
		})
	}
}
