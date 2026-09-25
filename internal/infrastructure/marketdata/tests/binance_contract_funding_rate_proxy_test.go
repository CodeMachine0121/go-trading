package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func settlementJson(fundingTime time.Time, rate string, markPrice string) string {
	return fmt.Sprintf(`{"symbol":"BTCUSDT","fundingTime":%d,"fundingRate":"%s","markPrice":"%s","rateType":"Regular"}`,
		fundingTime.UnixMilli(), rate, markPrice)
}

// fundingRateVenue answers each request with the next page and records each start time asked for.
type fundingRateVenue struct {
	url        string
	lock       *sync.Mutex
	startTimes *[]int64
}

func servedByFundingRateVenue(t *testing.T, pages ...string) fundingRateVenue {
	t.Helper()

	venue := fundingRateVenue{lock: &sync.Mutex{}, startTimes: &[]int64{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startTime, _ := strconv.ParseInt(request.URL.Query().Get("startTime"), 10, 64)
		venue.lock.Lock()
		*venue.startTimes = append(*venue.startTimes, startTime)
		asked := len(*venue.startTimes)
		venue.lock.Unlock()

		if asked > len(pages) {
			_, _ = writer.Write([]byte("[]"))

			return
		}
		_, _ = writer.Write([]byte(pages[asked-1]))
	}))
	t.Cleanup(server.Close)
	venue.url = server.URL

	return venue
}

func (venue fundingRateVenue) askedFrom() []int64 {
	venue.lock.Lock()
	defer venue.lock.Unlock()

	return append([]int64{}, *venue.startTimes...)
}

func settlementAt(hour int, millisecond int) time.Time {
	return time.Date(2026, 9, 23, hour, 0, 0, millisecond*int(time.Millisecond), time.UTC)
}

func TestFundingRateProxyReadsASettlementTheWayTheVenueStatedIt(t *testing.T) {
	venue := servedByFundingRateVenue(t,
		"["+settlementJson(settlementAt(8, 1), "-0.00003", "87000.5")+","+
			settlementJson(settlementAt(16, 0), "0", "")+"]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(venue.url, requestTimeout, unpaced())

	settlements, fetchError := fundingRateProxy.FetchFundingRateSettlements(
		t.Context(), "BTCUSDT", settlementAt(0, 0))

	require.NoError(t, fetchError)
	require.Len(t, settlements, 2)
	assert.Equal(t, "BTCUSDT", settlements[0].Symbol)
	assert.Equal(t, settlementAt(8, 1), settlements[0].SettlementTime)
	assert.True(t, decimal.RequireFromString("-0.00003").Equal(settlements[0].FundingRate))
	require.True(t, settlements[0].MarkPrice.Valid)
	assert.True(t, decimal.RequireFromString("87000.5").Equal(settlements[0].MarkPrice.Decimal))
	assert.True(t, decimal.Zero.Equal(settlements[1].FundingRate))
	assert.False(t, settlements[1].MarkPrice.Valid)
}

func TestFundingRateProxyAsksFromJustAfterTheLastHeldSettlement(t *testing.T) {
	venue := servedByFundingRateVenue(t, "[]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(venue.url, requestTimeout, unpaced())

	_, fetchError := fundingRateProxy.FetchFundingRateSettlements(t.Context(), "BTCUSDT", settlementAt(8, 0))

	require.NoError(t, fetchError)
	assert.Equal(t, []int64{settlementAt(8, 1).UnixMilli()}, venue.askedFrom())
}

func TestFundingRateProxyAsksFromBeforeEveryPerpetualWhenNothingIsHeld(t *testing.T) {
	venue := servedByFundingRateVenue(t, "[]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(venue.url, requestTimeout, unpaced())

	_, fetchError := fundingRateProxy.FetchFundingRateSettlements(t.Context(), "BTCUSDT", time.Time{})

	require.NoError(t, fetchError)
	assert.Equal(t, []int64{time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()}, venue.askedFrom())
}

func TestFundingRateProxyWalksEveryFullPage(t *testing.T) {
	firstPage := make([]string, 0, 1000)
	for index := range 1000 {
		firstPage = append(firstPage, settlementJson(
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(index)*8*time.Hour), "0.0001", "1"))
	}
	lastOfFirstPage := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Add(999 * 8 * time.Hour)
	venue := servedByFundingRateVenue(t,
		"["+strings.Join(firstPage, ",")+"]",
		"["+settlementJson(lastOfFirstPage.Add(8*time.Hour), "0.0002", "1")+"]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(venue.url, requestTimeout, unpaced())

	settlements, fetchError := fundingRateProxy.FetchFundingRateSettlements(t.Context(), "BTCUSDT", time.Time{})

	require.NoError(t, fetchError)
	assert.Len(t, settlements, 1001)
	askedFrom := venue.askedFrom()
	require.Len(t, askedFrom, 2)
	assert.Equal(t, lastOfFirstPage.Add(time.Millisecond).UnixMilli(), askedFrom[1])
}

func TestFundingRateProxyStopsWhenAFullPageDoesNotMoveForward(t *testing.T) {
	stuckPage := make([]string, 0, 1000)
	for range 1000 {
		stuckPage = append(stuckPage, settlementJson(time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), "0.0001", "1"))
	}
	venue := servedByFundingRateVenue(t, "["+strings.Join(stuckPage, ",")+"]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(venue.url, requestTimeout, unpaced())

	settlements, fetchError := fundingRateProxy.FetchFundingRateSettlements(t.Context(), "BTCUSDT", time.Time{})

	require.NoError(t, fetchError)
	assert.Len(t, settlements, 1000)
	assert.Len(t, venue.askedFrom(), 1)
}

func TestFundingRateProxySaysSoWhenTheVenueCannotBeReadOrRefuses(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "拒絕", statusCode: http.StatusTooManyRequests, body: ""},
		{name: "不是陣列", statusCode: http.StatusOK, body: `{"code":-1121}`},
		{name: "費率不是數字", statusCode: http.StatusOK, body: "[" + settlementJson(settlementAt(8, 0), "nope", "1") + "]"},
		{name: "標記價格不是數字", statusCode: http.StatusOK, body: "[" + settlementJson(settlementAt(8, 0), "0.1", "nope") + "]"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.statusCode)
				_, _ = writer.Write([]byte(testCase.body))
			}))
			t.Cleanup(server.Close)
			fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(server.URL, requestTimeout, unpaced())

			settlements, fetchError := fundingRateProxy.FetchFundingRateSettlements(
				t.Context(), "BTCUSDT", time.Time{})

			assert.Error(t, fetchError)
			assert.Nil(t, settlements)
		})
	}
}

func TestFundingRateProxySaysSoWhenTheVenueCannotBeReached(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1:1", controlCharacterUrl} {
		fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(address, requestTimeout, unpaced())

		_, fetchError := fundingRateProxy.FetchFundingRateSettlements(t.Context(), "BTCUSDT", time.Time{})

		assert.ErrorContains(t, fetchError, "reach contract funding rate source")
	}
}

func TestFundingRateProxyGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	venue := servedByFundingRateVenue(t, "[]")
	fundingRateProxy := marketdata.NewBinanceContractFundingRateProxy(
		venue.url, requestTimeout, marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, fetchError := fundingRateProxy.FetchFundingRateSettlements(abandonedContext, "BTCUSDT", time.Time{})

	assert.ErrorIs(t, fetchError, context.Canceled)
}
