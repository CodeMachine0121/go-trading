package marketdata_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const requestTimeout = 2 * time.Second

func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 30, hour, minute, 0, 0, time.UTC)
}

// kLineJson spells one K candle the way the source does: a positional array whose
// first element is a number of milliseconds and whose figures are quoted.
func kLineJson(openTime time.Time) string {
	return fmt.Sprintf(
		`[%d,"100","120","90","110","11",%d,"1200",7007,"5","600","0"]`,
		openTime.UnixMilli(), openTime.Add(5*time.Minute).UnixMilli()-1)
}

// servedBy answers every request with the given body.
func servedBy(t *testing.T, body string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func TestFetchKCandlesMapsEveryPositionOfTheSourceArray(t *testing.T) {
	baseUrl := servedBy(t, `[[1788019500000,"1","2","0.5","1.5","10",1788019799999,"2000",7007,"4","800","0"]]`)
	proxy := marketdata.NewBinanceMarketDataProxy(baseUrl, requestTimeout)

	reportedOpenTime := time.Unix(1788019500, 0).UTC()

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", reportedOpenTime, reportedOpenTime))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	marketKCandle := marketKCandles[0]
	assert.Equal(t, "BTCUSDT", marketKCandle.Symbol)
	assert.Equal(t, reportedOpenTime, marketKCandle.OpenTime)
	assert.Equal(t, "1", marketKCandle.Open.String())
	assert.Equal(t, "2", marketKCandle.High.String())
	assert.Equal(t, "0.5", marketKCandle.Low.String())
	assert.Equal(t, "1.5", marketKCandle.Close.String())
	assert.Equal(t, "10", marketKCandle.Volume.String())
	assert.Equal(t, "2000", marketKCandle.QuoteVolume.String())
	assert.Equal(t, "4", marketKCandle.TakerBuyBaseVolume.String())
	assert.Equal(t, "800", marketKCandle.TakerBuyQuoteVolume.String())
}

func TestFetchKCandlesAsksTheSourceForTheWindow(t *testing.T) {
	requestedQuery := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestedQuery <- map[string]string{
			"symbol":    request.URL.Query().Get("symbol"),
			"interval":  request.URL.Query().Get("interval"),
			"startTime": request.URL.Query().Get("startTime"),
			"endTime":   request.URL.Query().Get("endTime"),
		}
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	_, fetchError := proxy.FetchKCandles(t.Context(), vo.NewKCandleFetchWindowVo("BTCUSDT", at(8, 40), at(9, 0)))

	require.NoError(t, fetchError)
	query := <-requestedQuery
	assert.Equal(t, "BTCUSDT", query["symbol"])
	assert.Equal(t, "5m", query["interval"])
	assert.Equal(t, strconv.FormatInt(at(8, 40).UnixMilli(), 10), query["startTime"])
	assert.Equal(t, strconv.FormatInt(at(9, 0).UnixMilli(), 10), query["endTime"])
}

func TestFetchKCandlesKeepsAskingUntilTheWindowIsCovered(t *testing.T) {
	available := []time.Time{at(8, 40), at(8, 45), at(8, 50), at(8, 55), at(9, 0)}
	const pageSize = 2

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		startTimeMilliseconds, _ := strconv.ParseInt(request.URL.Query().Get("startTime"), 10, 64)
		startTime := time.UnixMilli(startTimeMilliseconds).UTC()

		page := make([]string, 0, pageSize)
		for _, openTime := range available {
			if !openTime.Before(startTime) && len(page) < pageSize {
				page = append(page, kLineJson(openTime))
			}
		}
		_, _ = writer.Write([]byte("[" + strings.Join(page, ",") + "]"))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(8, 40), at(9, 0)))

	require.NoError(t, fetchError)
	assert.Equal(t, available, openTimesOf(marketKCandles))
	assert.Greater(t, requestCount, 1)
}

func TestFetchKCandlesAcceptsFewerCandlesThanTheWindowCovers(t *testing.T) {
	answered := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if answered {
			_, _ = writer.Write([]byte(`[]`))
			return
		}
		answered = true
		_, _ = writer.Write([]byte("[" + kLineJson(at(8, 40)) + "," + kLineJson(at(8, 45)) + "]"))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(8, 40), at(9, 0)))

	require.NoError(t, fetchError)
	assert.Equal(t, []time.Time{at(8, 40), at(8, 45)}, openTimesOf(marketKCandles))
}

func TestFetchKCandlesTreatsNothingAvailableAsAnEmptyResult(t *testing.T) {
	proxy := marketdata.NewBinanceMarketDataProxy(servedBy(t, `[]`), requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(8, 40), at(9, 0)))

	require.NoError(t, fetchError)
	assert.Empty(t, marketKCandles)
}

func TestFetchKCandlesReportsAnUnusableAnswer(t *testing.T) {
	testCases := []struct {
		name           string
		body           string
		expectedReason string
	}{
		{
			name:           "not readable at all",
			body:           "not json",
			expectedReason: "read market source answer for BTCUSDT",
		},
		{
			name:           "a candle with fewer fields than the layout has",
			body:           `[[1788019500000,"1","2","0.5","1.5"]]`,
			expectedReason: "expected at least 11",
		},
		{
			name:           "an open time that is not a number of milliseconds",
			body:           `[["abc","1","2","0.5","1.5","10",1788019799999,"2000",7007,"4","800","0"]]`,
			expectedReason: "read open time from market source",
		},
		{
			name:           "a figure the source left unquoted",
			body:           `[[1788019500000,1,"2","0.5","1.5","10",1788019799999,"2000",7007,"4","800","0"]]`,
			expectedReason: "read figure at position 1",
		},
		{
			name:           "a figure that is not a number",
			body:           `[[1788019500000,"abc","2","0.5","1.5","10",1788019799999,"2000",7007,"4","800","0"]]`,
			expectedReason: "read figure at position 1",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proxy := marketdata.NewBinanceMarketDataProxy(servedBy(t, testCase.body), requestTimeout)

			marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
				vo.NewKCandleFetchWindowVo("BTCUSDT", at(9, 0), at(9, 0)))

			require.Error(t, fetchError)
			assert.Contains(t, fetchError.Error(), testCase.expectedReason)
			assert.Nil(t, marketKCandles)
		})
	}
}

func TestFetchKCandlesReportsASourceThatWillNotServe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(9, 0), at(9, 0)))

	require.Error(t, fetchError)
	assert.Contains(t, fetchError.Error(), "market source answered 500 for BTCUSDT")
	assert.Nil(t, marketKCandles)
}

func TestFetchKCandlesReportsASourceItCannotReach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	unreachableUrl := server.URL
	server.Close()
	proxy := marketdata.NewBinanceMarketDataProxy(unreachableUrl, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(9, 0), at(9, 0)))

	require.Error(t, fetchError)
	assert.Contains(t, fetchError.Error(), "reach market source for BTCUSDT")
	assert.Nil(t, marketKCandles)
}

func TestFetchKCandlesReportsAnAnswerItCannotFinishReading(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", "512")
		_, _ = writer.Write([]byte(`[`))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(9, 0), at(9, 0)))

	require.Error(t, fetchError)
	assert.Contains(t, fetchError.Error(), "read market source answer for BTCUSDT")
	assert.Nil(t, marketKCandles)
}

func openTimesOf(marketKCandles []vo.MarketKCandleVo) []time.Time {
	openTimes := make([]time.Time, 0, len(marketKCandles))
	for _, marketKCandle := range marketKCandles {
		openTimes = append(openTimes, marketKCandle.OpenTime)
	}

	return openTimes
}

func TestFetchKCandlesDiscardsCandlesOutsideTheWindowAndStopsAsking(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		_, _ = writer.Write([]byte("[" + kLineJson(at(9, 30)) + "]"))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceMarketDataProxy(server.URL, requestTimeout)

	marketKCandles, fetchError := proxy.FetchKCandles(t.Context(),
		vo.NewKCandleFetchWindowVo("BTCUSDT", at(8, 40), at(9, 0)))

	require.NoError(t, fetchError)
	assert.Empty(t, marketKCandles)
	assert.Equal(t, 1, requestCount)
}
