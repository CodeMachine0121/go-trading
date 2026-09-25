package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tradedKLineJson has the trade count unquoted among quoted figures, as the venue sends it.
func tradedKLineJson(openTime time.Time, tradeCount int) string {
	return fmt.Sprintf(
		`[%d,"85570.30","85594.40","85570.30","85572.00","55.714",%d,"4767877.86870",%d,"34.469","2949728.83040","0"]`,
		openTime.UnixMilli(), openTime.Add(time.Minute).UnixMilli()-1, tradeCount)
}

// markPriceKLineJson carries four real prices and placeholder zeros where volumes would be.
func markPriceKLineJson(openTime time.Time, closePrice string) string {
	return fmt.Sprintf(
		`[%d,"85573.82868116","85594.40000000","85572.77281159","%s","0",%d,"0",60,"0","0","0"]`,
		openTime.UnixMilli(), closePrice, openTime.Add(time.Minute).UnixMilli()-1)
}

func indexPriceKLineJson(openTime time.Time, closePrice string) string {
	return fmt.Sprintf(
		`[%d,"87248.06217391","87268.33000000","87220.35326087","%s","0",%d,"0",48,"0","0","0"]`,
		openTime.UnixMilli(), closePrice, openTime.Add(time.Minute).UnixMilli()-1)
}

func premiumIndexKLineJson(openTime time.Time, closeFigure string) string {
	return fmt.Sprintf(
		`[%d,"-0.00049929","-0.00038347","-0.00071252","%s","0",%d,"0",10,"0","0","0"]`,
		openTime.UnixMilli(), closeFigure, openTime.Add(time.Minute).UnixMilli()-1)
}

// contractVenue serves the four endpoints separately, counting requests and recording the last query to each.
type contractVenue struct {
	tradedUrl         string
	markPriceUrl      string
	indexPriceUrl     string
	premiumIndexUrl   string
	tradedCalls       *atomic.Int32
	markPriceCall     *atomic.Int32
	indexPriceCalls   *atomic.Int32
	premiumIndexCalls *atomic.Int32
	indexPriceQuery   *atomic.Value
	premiumIndexQuery *atomic.Value
}

// servedByContractVenue answers each address once then empty so paging stops; index and premium answer empty unless given a body.
func servedByContractVenue(
	t *testing.T, tradedBody string, markPriceBody string, laterLineBodies ...string,
) contractVenue {
	t.Helper()

	indexPriceBody, premiumIndexBody := "[]", "[]"
	if len(laterLineBodies) == 2 {
		indexPriceBody, premiumIndexBody = laterLineBodies[0], laterLineBodies[1]
	}

	venue := contractVenue{
		tradedCalls:       &atomic.Int32{},
		markPriceCall:     &atomic.Int32{},
		indexPriceCalls:   &atomic.Int32{},
		premiumIndexCalls: &atomic.Int32{},
		indexPriceQuery:   &atomic.Value{},
		premiumIndexQuery: &atomic.Value{},
	}
	answerOnce := func(calls *atomic.Int32, body string, lastQuery *atomic.Value) http.HandlerFunc {
		return func(writer http.ResponseWriter, request *http.Request) {
			if lastQuery != nil {
				lastQuery.Store(request.URL.Query())
			}
			if calls.Add(1) > 1 {
				_, _ = writer.Write([]byte("[]"))

				return
			}
			_, _ = writer.Write([]byte(body))
		}
	}
	requestMultiplexer := http.NewServeMux()
	requestMultiplexer.HandleFunc("/klines", answerOnce(venue.tradedCalls, tradedBody, nil))
	requestMultiplexer.HandleFunc("/markPriceKlines",
		answerOnce(venue.markPriceCall, markPriceBody, nil))
	requestMultiplexer.HandleFunc("/indexPriceKlines",
		answerOnce(venue.indexPriceCalls, indexPriceBody, venue.indexPriceQuery))
	requestMultiplexer.HandleFunc("/premiumIndexKlines",
		answerOnce(venue.premiumIndexCalls, premiumIndexBody, venue.premiumIndexQuery))
	server := httptest.NewServer(requestMultiplexer)
	t.Cleanup(server.Close)

	venue.tradedUrl = server.URL + "/klines"
	venue.markPriceUrl = server.URL + "/markPriceKlines"
	venue.indexPriceUrl = server.URL + "/indexPriceKlines"
	venue.premiumIndexUrl = server.URL + "/premiumIndexKlines"

	return venue
}

func (venue contractVenue) proxyFor(pacer marketdata.RequestPacer) *marketdata.BinanceContractMarketDataProxy {
	return marketdata.NewBinanceContractMarketDataProxy(
		venue.tradedUrl, venue.markPriceUrl, venue.indexPriceUrl, venue.premiumIndexUrl,
		requestTimeout, pacer)
}

func contractWindow(startTime time.Time, endTime time.Time) vo.KCandleFetchWindowVo {
	return vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, startTime, endTime)
}

func TestContractProxyMergesTheTwoAnswersIntoOneCandle(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574.49072464")+"]")
	contractProxy := venue.proxyFor(unpaced())

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
	contractProxy := venue.proxyFor(unpaced())

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
	contractProxy := venue.proxyFor(unpaced())

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
		server.URL+"/klines", server.URL+"/markPriceKlines",
		server.URL+"/indexPriceKlines", server.URL+"/premiumIndexKlines", requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.Error(t, fetchError)
	assert.Nil(t, contractKCandles)
}

func TestContractProxyDoesNotAskForMarkPricesWhenNothingTraded(t *testing.T) {
	venue := servedByContractVenue(t, "[]", "["+markPriceKLineJson(at(9, 0), "85574")+"]")
	contractProxy := venue.proxyFor(unpaced())

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
	contractProxy := venue.proxyFor(unpaced())

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
			contractProxy := venue.proxyFor(unpaced())

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
		server.URL, server.URL, server.URL, server.URL, requestTimeout, unpaced())

	_, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	assert.ErrorContains(t, fetchError, "429")
}

func TestContractProxySaysSoWhenTheSourceCannotBeReached(t *testing.T) {
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		"http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1",
		requestTimeout, unpaced())

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
	for _, laterLine := range []string{"/markPriceKlines", "/indexPriceKlines", "/premiumIndexKlines"} {
		requestMultiplexer.HandleFunc(laterLine, func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("[]"))
		})
	}
	server := httptest.NewServer(requestMultiplexer)
	t.Cleanup(server.Close)
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		server.URL+"/klines", server.URL+"/markPriceKlines",
		server.URL+"/indexPriceKlines", server.URL+"/premiumIndexKlines", requestTimeout, unpaced())

	contractKCandles, fetchError := contractProxy.FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 5)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 2)
	assert.Equal(t, at(9, 0), contractKCandles[0].OpenTime)
	assert.Equal(t, at(9, 1), contractKCandles[1].OpenTime)
}

// controlCharacterUrl cannot be turned into a request, reaching the failure before anything is sent.
const controlCharacterUrl = "http://\x7f"

func TestContractProxyGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	venue := servedByContractVenue(t, "["+tradedKLineJson(at(9, 0), 1)+"]", "[]")
	contractProxy := venue.proxyFor(marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, fetchError := contractProxy.FetchKCandles(
		abandonedContext, contractWindow(at(9, 0), at(9, 0)))

	assert.ErrorIs(t, fetchError, context.Canceled)
}

func TestContractProxySaysSoWhenTheAddressCannotBeTurnedIntoARequest(t *testing.T) {
	contractProxy := marketdata.NewBinanceContractMarketDataProxy(
		controlCharacterUrl, controlCharacterUrl, controlCharacterUrl, controlCharacterUrl,
		requestTimeout, unpaced())

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
			contractProxy := venue.proxyFor(unpaced())

			_, fetchError := contractProxy.FetchKCandles(
				t.Context(), contractWindow(at(9, 0), at(9, 0)))

			assert.Error(t, fetchError)
		})
	}
}

func TestContractProxyMergesTheIndexPriceAndPremiumIndexIntoTheCandle(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+"]",
		"["+markPriceKLineJson(at(9, 0), "85574.49072464")+"]",
		"["+indexPriceKLineJson(at(9, 0), "87261.48869565")+"]",
		"["+premiumIndexKLineJson(at(9, 0), "-0.00051123")+"]")

	contractKCandles, fetchError := venue.proxyFor(unpaced()).FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 1)
	fetchedCandle := contractKCandles[0]
	require.True(t, fetchedCandle.IndexClose.Valid)
	assert.True(t, decimal.RequireFromString("87248.06217391").Equal(fetchedCandle.IndexOpen.Decimal))
	assert.True(t, decimal.RequireFromString("87268.33000000").Equal(fetchedCandle.IndexHigh.Decimal))
	assert.True(t, decimal.RequireFromString("87220.35326087").Equal(fetchedCandle.IndexLow.Decimal))
	assert.True(t, decimal.RequireFromString("87261.48869565").Equal(fetchedCandle.IndexClose.Decimal))
	require.True(t, fetchedCandle.PremiumIndexClose.Valid)
	assert.True(t, decimal.RequireFromString("-0.00049929").Equal(fetchedCandle.PremiumIndexOpen.Decimal))
	assert.True(t, decimal.RequireFromString("-0.00038347").Equal(fetchedCandle.PremiumIndexHigh.Decimal))
	assert.True(t, decimal.RequireFromString("-0.00071252").Equal(fetchedCandle.PremiumIndexLow.Decimal))
	assert.True(t, decimal.RequireFromString("-0.00051123").Equal(fetchedCandle.PremiumIndexClose.Decimal))
	// Volume comes from the traded answer, not the index answer's placeholder zeros.
	assert.True(t, decimal.RequireFromString("55.714").Equal(fetchedCandle.Volume))
}

func TestContractProxyLeavesThePremiumIndexAbsentWhenThatMinuteIsMissing(t *testing.T) {
	venue := servedByContractVenue(t,
		"["+tradedKLineJson(at(9, 0), 2541)+","+tradedKLineJson(at(9, 1), 2000)+"]",
		"["+markPriceKLineJson(at(9, 0), "1")+","+markPriceKLineJson(at(9, 1), "1")+"]",
		"["+indexPriceKLineJson(at(9, 0), "1")+","+indexPriceKLineJson(at(9, 1), "1")+"]",
		"["+premiumIndexKLineJson(at(9, 0), "0")+"]")

	contractKCandles, fetchError := venue.proxyFor(unpaced()).FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 1)))

	require.NoError(t, fetchError)
	require.Len(t, contractKCandles, 2)
	assert.True(t, contractKCandles[0].PremiumIndexClose.Valid)
	assert.True(t, contractKCandles[1].IndexClose.Valid)
	assert.True(t, contractKCandles[1].MarkClose.Valid)
	assert.False(t, contractKCandles[1].PremiumIndexOpen.Valid)
	assert.False(t, contractKCandles[1].PremiumIndexHigh.Valid)
	assert.False(t, contractKCandles[1].PremiumIndexLow.Valid)
	assert.False(t, contractKCandles[1].PremiumIndexClose.Valid)
}

func TestContractProxyFailsWhenTheIndexPriceOrPremiumIndexAnswerCannotBeRead(t *testing.T) {
	for _, failingLine := range []string{"/indexPriceKlines", "/premiumIndexKlines"} {
		t.Run(failingLine, func(t *testing.T) {
			requestMultiplexer := http.NewServeMux()
			requestMultiplexer.HandleFunc("/klines", func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte("[" + tradedKLineJson(at(9, 0), 2541) + "]"))
			})
			for _, line := range []string{"/markPriceKlines", "/indexPriceKlines", "/premiumIndexKlines"} {
				requestMultiplexer.HandleFunc(line, func(writer http.ResponseWriter, _ *http.Request) {
					if line == failingLine {
						writer.WriteHeader(http.StatusInternalServerError)

						return
					}
					_, _ = writer.Write([]byte("[]"))
				})
			}
			server := httptest.NewServer(requestMultiplexer)
			t.Cleanup(server.Close)
			contractProxy := marketdata.NewBinanceContractMarketDataProxy(
				server.URL+"/klines", server.URL+"/markPriceKlines",
				server.URL+"/indexPriceKlines", server.URL+"/premiumIndexKlines",
				requestTimeout, unpaced())

			contractKCandles, fetchError := contractProxy.FetchKCandles(
				t.Context(), contractWindow(at(9, 0), at(9, 0)))

			assert.ErrorContains(t, fetchError, "500")
			assert.Nil(t, contractKCandles)
		})
	}
}

func TestContractProxyAsksForNeitherLaterLineWhenNothingTraded(t *testing.T) {
	venue := servedByContractVenue(t, "[]", "[]",
		"["+indexPriceKLineJson(at(9, 0), "1")+"]", "["+premiumIndexKLineJson(at(9, 0), "0")+"]")

	contractKCandles, fetchError := venue.proxyFor(unpaced()).FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 5)))

	require.NoError(t, fetchError)
	assert.Empty(t, contractKCandles)
	assert.Equal(t, int32(0), venue.indexPriceCalls.Load())
	assert.Equal(t, int32(0), venue.premiumIndexCalls.Load())
}

func TestContractProxyNamesThePairForTheIndexPriceAndTheSymbolForThePremium(t *testing.T) {
	venue := servedByContractVenue(t, "["+tradedKLineJson(at(9, 0), 1)+"]", "[]")

	_, fetchError := venue.proxyFor(unpaced()).FetchKCandles(
		t.Context(), contractWindow(at(9, 0), at(9, 0)))

	require.NoError(t, fetchError)
	indexPriceQuery := venue.indexPriceQuery.Load().(url.Values)
	assert.Equal(t, "BTCUSDT", indexPriceQuery.Get("pair"))
	assert.Empty(t, indexPriceQuery.Get("symbol"))
	premiumIndexQuery := venue.premiumIndexQuery.Load().(url.Values)
	assert.Equal(t, "BTCUSDT", premiumIndexQuery.Get("symbol"))
	assert.Empty(t, premiumIndexQuery.Get("pair"))
}
