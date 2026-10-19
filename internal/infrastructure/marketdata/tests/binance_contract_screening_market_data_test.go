package marketdata_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fourHourKLineJson(openTime time.Time, closePrice string) string {
	return fmt.Sprintf(`[%d,"100","110","90","%s","55.7",%d,"5570",12,"30","3000","0"]`,
		openTime.UnixMilli(), closePrice, openTime.Add(4*time.Hour).UnixMilli()-1)
}

func fourHourMarkJson(openTime time.Time, closePrice string) string {
	return fmt.Sprintf(`[%d,"100","111","89","%s","0",%d,"0",240,"0","0","0"]`,
		openTime.UnixMilli(), closePrice, openTime.Add(4*time.Hour).UnixMilli()-1)
}

func TestContractProxyFetchesTheVenuesOwnFourHourCandlesWithTheirMarkPrice(t *testing.T) {
	firstOpen := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	venue := servedByContractVenue(t,
		"["+fourHourKLineJson(firstOpen, "105")+","+fourHourKLineJson(firstOpen.Add(4*time.Hour), "107")+"]",
		"["+fourHourMarkJson(firstOpen, "104.5")+"]")

	kCandles, fetchError := venue.proxyFor(unpaced()).FetchAggregatedKCandles(t.Context(), vo.AggregatedKCandleFetchWindowVo{
		Symbol: "LINKUSDT", Interval: "4h", Step: 4 * time.Hour,
		StartTime: firstOpen, EndTime: firstOpen.Add(8*time.Hour - time.Millisecond),
	})

	require.NoError(t, fetchError)
	require.Len(t, kCandles, 2)
	assert.Equal(t, firstOpen.Add(4*time.Hour), kCandles[1].OpenTime)
	assert.True(t, decimal.RequireFromString("5570").Equal(kCandles[0].QuoteVolume))
	require.True(t, kCandles[0].MarkClose.Valid)
	assert.True(t, decimal.RequireFromString("104.5").Equal(kCandles[0].MarkClose.Decimal))
	assert.False(t, kCandles[1].MarkClose.Valid)
	assert.False(t, kCandles[0].IndexClose.Valid)
	assert.Equal(t, int32(0), venue.indexPriceCalls.Load())
}

func TestContractSymbolLookupKnowsAPerpetualTheVenueNoLongerTrades(t *testing.T) {
	testCases := []struct {
		name                   string
		symbol                 string
		expectedIsListed       bool
		expectedIsKnownToVenue bool
	}{
		{name: "trading", symbol: "BTCUSDT", expectedIsListed: true, expectedIsKnownToVenue: true},
		{name: "stopped trading", symbol: "OMGUSDT", expectedIsListed: false, expectedIsKnownToVenue: true},
		{name: "never there", symbol: "NOSUCHPAIR", expectedIsListed: false, expectedIsKnownToVenue: false},
		{name: "a dated future is not a perpetual", symbol: "BTCUSDT_260925", expectedIsListed: false,
			expectedIsKnownToVenue: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
				servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

			listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), testCase.symbol)

			require.NoError(t, lookupError)
			assert.Equal(t, testCase.expectedIsListed, listing.IsListed)
			assert.Equal(t, testCase.expectedIsKnownToVenue, listing.IsKnownToVenue)
		})
	}
}
