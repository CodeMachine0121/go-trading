package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tradedKLineJson spells the traded half of a contract candle the way the source
// does, with the number of trades sitting unquoted among the quoted figures.
func tradedKLineJson(openTime time.Time, tradeCount int) string {
	return fmt.Sprintf(
		`[%d,"85570.30","85594.40","85570.30","85572.00","55.714",%d,"4767877.86870",%d,"34.469","2949728.83040","0"]`,
		openTime.UnixMilli(), openTime.Add(time.Minute).UnixMilli()-1, tradeCount)
}

// markPriceKLineJson spells a mark price answer the way the source does: four real
// prices, and placeholder zeros where a traded answer carries its volumes.
func markPriceKLineJson(openTime time.Time, closePrice string) string {
	return fmt.Sprintf(
		`[%d,"85573.82868116","85594.40000000","85572.77281159","%s","0",%d,"0",60,"0","0","0"]`,
		openTime.UnixMilli(), closePrice, openTime.Add(time.Minute).UnixMilli()-1)
}

// contractVenue answers the traded address and the mark price address separately,
// and counts how often each was asked.
type contractVenue struct {
	tradedUrl     string
	markPriceUrl  string
	tradedCalls   *atomic.Int32
	markPriceCall *atomic.Int32
}

func servedByContractVenue(t *testing.T, tradedBody string, markPriceBody string) contractVenue {
	t.Helper()

	tradedCalls := &atomic.Int32{}
	markPriceCalls := &atomic.Int32{}
	requestMultiplexer := http.NewServeMux()
	requestMultiplexer.HandleFunc("/klines", func(writer http.ResponseWriter, _ *http.Request) {
		if tradedCalls.Add(1) > 1 {
			_, _ = writer.Write([]byte("[]"))

			return
		}
		_, _ = writer.Write([]byte(tradedBody))
	})
	requestMultiplexer.HandleFunc("/markPriceKlines", func(writer http.ResponseWriter, _ *http.Request) {
		if markPriceCalls.Add(1) > 1 {
			_, _ = writer.Write([]byte("[]"))

			return
		}
		_, _ = writer.Write([]byte(markPriceBody))
	})
	server := httptest.NewServer(requestMultiplexer)
	t.Cleanup(server.Close)

	return contractVenue{
		tradedUrl:     server.URL + "/klines",
		markPriceUrl:  server.URL + "/markPriceKlines",
		tradedCalls:   tradedCalls,
		markPriceCall: markPriceCalls,
	}
}

func contractWindow(startTime time.Time, endTime time.Time) vo.KCandleFetchWindowVo {
	return vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, startTime, endTime)
}

func TestContractProxyMergesTheTwoAnswersIntoOneCandle(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574.49072464")+"]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 1)
	fetchedCandle := contractKCandles[0]
	assert.Equal(t, "BTCUSDT", fetchedCandle.Symbol)
	assert.Equal(t, at(9, 0), fetchedCandle.OpenTime)
	assert.True(t, decimal.RequireFromString("85572.00").Equal(fetchedCandle.Close))
	assert.Equal(t, int64(2541), fetchedCandle.TradeCount)
	require.True(t, fetchedCandle.MarkClose.Valid)
	assert.True(t, decimal.RequireFromString("85574.49072464").Equal(fetchedCandle.MarkClose.Decimal))
	assert.True(t, decimal.RequireFromString("85573.82868116").Equal(fetchedCandle.MarkOpen.Decimal))
	assert.True(t, decimal.RequireFromString("85594.40000000").Equal(fetchedCandle.MarkHigh.Decimal))
	assert.True(t, decimal.RequireFromString("85572.77281159").Equal(fetchedCandle.MarkLow.Decimal))
}

func TestContractProxyDropsThePlaceholderZerosOnTheMarkPriceAnswer(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574.49072464")+"]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 1)
	fetchedCandle := contractKCandles[0]
	assert.True(t, decimal.RequireFromString("55.714").Equal(fetchedCandle.Volume))
	assert.True(t, decimal.RequireFromString("4767877.86870").Equal(fetchedCandle.QuoteVolume))
	assert.True(t, decimal.RequireFromString("34.469").Equal(fetchedCandle.TakerBuyBaseVolume))
	assert.True(t, decimal.RequireFromString("2949728.83040").Equal(fetchedCandle.TakerBuyQuoteVolume))
	assert.Equal(t, int64(2541), fetchedCandle.TradeCount)
}

func TestContractProxyLeavesTheMarkPriceAbsentWhenThatMinuteIsMissing(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+","+tradedKLineJson(at(9, 1), 2000)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574.49072464")+"]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 1)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 2)
	assert.True(t, contractKCandles[0].MarkClose.Valid)
	assert.False(t, contractKCandles[1].MarkOpen.Valid)
	assert.False(t, contractKCandles[1].MarkHigh.Valid)
	assert.False(t, contractKCandles[1].MarkLow.Valid)
	assert.False(t, contractKCandles[1].MarkClose.Valid)
}

func TestContractProxyFailsWhenTheMarkPriceAnswerCannotBeRead(t *testing.T) {
	requestMultiplexer := http.NewServeMux()
	requestMultiplexer.HandleFunc("/klines", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("[" + tradedKLineJson(at(9, 0), 2541) + "]"))
	})
	requestMultiplexer.HandleFunc("/markPriceKlines", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(requestMultiplexer)
	t.Cleanup(server.Close)
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		server.URL+"/klines", server.URL+"/markPriceKlines", requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.Error(t, fetchError)
	assert.Nil(t, contractKCandles)
}

func TestContractProxyDoesNotAskForMarkPricesWhenNothingTraded(t *testing.T) {
	venue := servedByContractVenue(t, "[]", "["+markPriceKLineJson(at(9, 0), "85574")+"]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 5)))

	require.NoError(t, fetchError)
	assert.Empty(t, contractKCandles)
	assert.Equal(t, int32(0), venue.markPriceCall.Load())
}

func TestContractProxyKeepsOnlyTheRowsInsideTheWindow(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(8, 59), 1)+","+tradedKLineJson(at(9, 0), 2541)+","+
			tradedKLineJson(at(9, 5), 3)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574")+"]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 1)
	assert.Equal(t, at(9, 0), contractKCandles[0].OpenTime)
}

func TestContractProxySaysSoWhenTheSourceCannotBeRead(t *testing.T) {
	testCases := []struct {
		name       string
		tradedBody string
		statusCode int
	}{
		{name: "答案後面還有東西", tradedBody: "[]{}", statusCode: http.StatusOK},
		{name: "答案不是陣列", tradedBody: `{"code":-1121}`, statusCode: http.StatusOK},
		{
			name: "成交筆數不是數字",
			tradedBody: fmt.Sprintf(
				`[[%d,"1","1","1","1","1",%d,"1","not-a-number","1","1","0"]]`,
				at(9, 0).UnixMilli(), at(9, 1).UnixMilli()-1),
			statusCode: http.StatusOK,
		},
		{
			name:       "欄位不夠",
			tradedBody: fmt.Sprintf(`[[%d,"1","1"]]`, at(9, 0).UnixMilli()),
			statusCode: http.StatusOK,
		},
		{
			name:       "起始時刻不是數字",
			tradedBody: `[["not-a-time","1","1","1","1","1",1,"1",1,"1","1","0"]]`,
			statusCode: http.StatusOK,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			venue := servedByContractVenue(t, testCase.tradedBody, "[]")
			contractProxy := marketdata.NewBinanceContractMarketDataProxy(
				venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

			_, fetchError := contractProxy.FetchKCandles(
				t.Context(), contractWindow(at(9, 0), at(9, 0)))

			assert.Error(t, fetchError)
		})
	}
}

func TestContractProxySaysSoWhenTheSourceRefuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusTooManyRequests)
		}))
	t.Cleanup(server.Close)
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		server.URL, server.URL, requestTimeout, unpaced())

	_, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.ErrorContains(t, fetchError, "429")
}

func TestContractProxySaysSoWhenTheSourceCannotBeReached(t *testing.T) {
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		"http://127.0.0.1:1", "http://127.0.0.1:1", requestTimeout, unpaced())

	_, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.Error(t, fetchError)
}

func TestContractProxyWalksAWindowWiderThanOnePage(t *testing.T) {
	tradedCalls := &atomic.Int32{}
	requestMultiplexer := http.NewServeMux()
	requestMultiplexer.HandleFunc("/klines", func(writer http.ResponseWriter, _ *http.Request) {
		if tradedCalls.Add(1) == 1 {
			_, _ = writer.Write([]byte("[" + tradedKLineJson(at(9, 0), 1) + "]"))

			return
		}
		if tradedCalls.Load() == 2 {
			_, _ = writer.Write([]byte("[" + tradedKLineJson(at(9, 1), 2) + "]"))

			return
		}
		_, _ = writer.Write([]byte("[]"))
	})
	requestMultiplexer.HandleFunc("/markPriceKlines", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("[]"))
	})
	server := httptest.NewServer(requestMultiplexer)
	t.Cleanup(server.Close)
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		server.URL+"/klines", server.URL+"/markPriceKlines", requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 5)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 2)
	assert.Equal(t, at(9, 0), contractKCandles[0].OpenTime)
	assert.Equal(t, at(9, 1), contractKCandles[1].OpenTime)
}

// controlCharacterUrl cannot be turned into a request at all, which is the only way
// to reach the failure that happens before anything is sent.
const controlCharacterUrl = "http://\x7f"

func TestContractProxyGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	venue := servedByContractVenue(t, "["+tradedKLineJson(at(9, 0), 1)+"]", "[]")
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, requestTimeout, marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, fetchError := contractProxy.FetchKCandles(
		abandonedContext, contractWindow(at(9, 0), at(9, 0)))

	assert.ErrorIs(t, fetchError, context.Canceled)
}

func TestContractProxySaysSoWhenTheAddressCannotBeTurnedIntoARequest(t *testing.T) {
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		controlCharacterUrl, controlCharacterUrl, requestTimeout, unpaced())

	_, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.ErrorContains(t, fetchError, "reach contract market source")
}

func TestContractProxySaysSoWhenTheMarkPriceAnswerIsMalformed(t *testing.T) {
	testCases := []struct {
		name          string
		markPriceBody func() string
	}{
		{
			name: "標記價格的欄位不夠",
			markPriceBody: func() string {
				return fmt.Sprintf(`[[%d,"1","1"]]`, at(9, 0).UnixMilli())
			},
		},
		{
			name: "標記價格不是數字",
			markPriceBody: func() string {
				return fmt.Sprintf(
					`[[%d,"nope","1","1","1","0",%d,"0",60,"0","0","0"]]`,
					at(9, 0).UnixMilli(), at(9, 1).UnixMilli()-1)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			venue := servedByContractVenue(t,
				"["+tradedKLineJson(at(9, 0), 2541)+"]", testCase.markPriceBody())
			contractProxy := marketdata.NewBinanceContractMarketDataProxy(
				venue.tradedUrl, venue.markPriceUrl, requestTimeout, unpaced())

			_, fetchError := contractProxy.FetchKCandles(
				t.Context(), contractWindow(at(9, 0), at(9, 0)))

			assert.Error(t, fetchError)
		})
	}
}
