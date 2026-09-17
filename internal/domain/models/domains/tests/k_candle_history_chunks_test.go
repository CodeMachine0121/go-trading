package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyTaipei is the zone the Taiwan session below is said in.
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
	// One day is the unit for three reasons at once: one source is asked a local day
	// at a time, the coarsest aggregation bucket is a day, and it is the unit anybody
	// asking for history is thinking in.
	//
	// **Oldest first matters.** A run that dies half way then leaves a continuous
	// block of old candles with the gap at the recent end — which is exactly the shape
	// the ordinary backfill closes. Newest first would leave the gap in the middle,
	// and nothing in this system fills those.
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
	// No gap between one chunk and the next, and no overlap: a gap is a minute nobody
	// ever fetches, and an overlap is a request paid for twice.
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
	// It is the inclusive count, because that is what "how many candles should this
	// stretch hold" means — and comparing it against what storage holds is how a day
	// already complete is skipped without touching the source.
	testCases := []struct {
		name          string
		market        vo.MarketVo
		startTime     time.Time
		endTime       time.Time
		expectedCount int
	}{
		{
			// A round-the-clock market holds every minute of the day, both ends
			// included. TradingBucketCountBetween answers 1439 here — one short —
			// and that reading is left exactly as it is; this one is the inclusive
			// question, asked separately.
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
