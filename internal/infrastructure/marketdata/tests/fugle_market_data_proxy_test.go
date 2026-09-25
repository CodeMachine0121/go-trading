package marketdata_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// taipei is a fixed zone so tests do not depend on the machine's time zone database.
var taipei = time.FixedZone("Asia/Taipei", 8*60*60)

func taipeiMarket() domains.MarketDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   taipei,
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
		},
	}).MarketOf(string(vo.MarketTaiwanStock))
}

func taipeiAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

// fugleCandleJson carries its own offset and figures as plain JSON numbers.
func fugleCandleJson(localTime string) string {
	return fmt.Sprintf(
		`{"date":"%s","open":574,"high":576,"low":572,"close":575,"volume":8450,"average":573.82}`,
		localTime)
}

func fugleAnswerJson(symbol string, candles ...string) string {
	joined := ""
	for index, candle := range candles {
		if index > 0 {
			joined += ","
		}
		joined += candle
	}

	return fmt.Sprintf(`{"symbol":"%s","timeframe":"1","data":[%s]}`, symbol, joined)
}

type fugleSourceUnderTest struct {
	server *httptest.Server

	mutex             sync.Mutex
	intradayRequests  []*http.Request
	historicalRequest []*http.Request
	answers           map[string]string
}

func newFugleSourceUnderTest(t *testing.T) *fugleSourceUnderTest {
	t.Helper()

	source := &fugleSourceUnderTest{answers: make(map[string]string)}
	source.server = httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			source.mutex.Lock()
			if request.URL.Path[:len("/intraday")] == "/intraday" {
				source.intradayRequests = append(source.intradayRequests, request)
			} else {
				source.historicalRequest = append(source.historicalRequest, request)
			}
			// Unconfigured days answer empty so a test cannot pass on candles it never asked for.
			answer, hasAnswer := source.answers[request.URL.Query().Get("from")]
			source.mutex.Unlock()

			if !hasAnswer {
				answer = fugleAnswerJson("2330")
			}
			_, _ = writer.Write([]byte(answer))
		}))
	t.Cleanup(source.server.Close)

	return source
}

func (source *fugleSourceUnderTest) answersWith(fromDate string, body string) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	source.answers[fromDate] = body
}

func (source *fugleSourceUnderTest) askedIntraday() []*http.Request {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	return source.intradayRequests
}

func (source *fugleSourceUnderTest) askedHistorical() []*http.Request {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	return source.historicalRequest
}

func (source *fugleSourceUnderTest) proxyAt(
	t *testing.T, currentTime time.Time,
) *marketdata.FugleMarketDataProxy {
	t.Helper()

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	return marketdata.NewFugleMarketDataProxy(
		source.server.URL+"/intraday", source.server.URL+"/historical",
		"a-key", taipeiMarket(), clockProxy, requestTimeout, unpaced())
}

func fugleWindow(t *testing.T, startTime string, endTime string) vo.KCandleFetchWindowVo {
	t.Helper()

	return vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock, taipeiAt(t, startTime), taipeiAt(t, endTime))
}

func TestFugleReadsAPushedCandleIntoTheShapeTheDomainKnows(t *testing.T) {
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330", fugleCandleJson("2026-09-08T10:00:00.000+08:00")))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	assert.Equal(t, "2330", marketKCandles[0].Symbol)
	// The source states its own offset; open times are kept in UTC.
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, "574", marketKCandles[0].Open.String())
	assert.Equal(t, "576", marketKCandles[0].High.String())
	assert.Equal(t, "572", marketKCandles[0].Low.String())
	assert.Equal(t, "575", marketKCandles[0].Close.String())
	// Volume stays in shares as reported.
	assert.Equal(t, "8450", marketKCandles[0].Volume.String())
}

func TestFugleLeavesTheFiguresItDoesNotPublishAbsent(t *testing.T) {
	// Absent is not zero: zero would claim no turnover.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330", fugleCandleJson("2026-09-08T10:00:00.000+08:00")))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	assert.False(t, marketKCandles[0].QuoteVolume.Valid)
	assert.False(t, marketKCandles[0].TakerBuyBaseVolume.Valid)
	assert.False(t, marketKCandles[0].TakerBuyQuoteVolume.Valid)
}

func TestFugleAsksTheAddressThatAnswersAboutTheDayWanted(t *testing.T) {
	// Today must use the intraday address; the historical one can return empty for today.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330"))

	_, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, source.askedIntraday(), 1)
	assert.Empty(t, source.askedHistorical())
	assert.Equal(t, "1", source.askedIntraday()[0].URL.Query().Get("timeframe"))
	// The source answers newest first unless told otherwise.
	assert.Equal(t, "asc", source.askedIntraday()[0].URL.Query().Get("sort"))
	assert.Equal(t, "a-key", source.askedIntraday()[0].Header.Get("X-API-KEY"))
}

func TestFugleAsksTheHistoricalAddressForAnEarlierDay(t *testing.T) {
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330"))

	_, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-12T09:00:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-11T09:00:00+08:00", "2026-09-11T13:25:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, source.askedHistorical(), 1)
	assert.Empty(t, source.askedIntraday())
	assert.Equal(t, "2026-09-11", source.askedHistorical()[0].URL.Query().Get("from"))
	assert.Equal(t, "2026-09-11", source.askedHistorical()[0].URL.Query().Get("to"))
}

func TestFugleAsksEachLocalDayAWindowSpansAndMergesTheAnswers(t *testing.T) {
	source := newFugleSourceUnderTest(t)
	source.answersWith("2026-09-11",
		fugleAnswerJson("2330", fugleCandleJson("2026-09-11T13:25:00.000+08:00")))
	source.answersWith("",
		fugleAnswerJson("2330", fugleCandleJson("2026-09-14T09:00:00.000+08:00")))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-14T09:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-11T09:00:00+08:00", "2026-09-14T09:05:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 2)
	assert.Equal(t, taipeiAt(t, "2026-09-11T13:25:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, taipeiAt(t, "2026-09-14T09:00:00+08:00").UTC(), marketKCandles[1].OpenTime)
}

func TestFugleKeepsOnlyTheCandlesInsideTheWindow(t *testing.T) {
	// The source answers whole days, so out-of-window candles must be trimmed.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330",
		fugleCandleJson("2026-09-08T09:00:00.000+08:00"),
		fugleCandleJson("2026-09-08T09:55:00.000+08:00"),
		fugleCandleJson("2026-09-08T10:00:00.000+08:00"),
		fugleCandleJson("2026-09-08T10:05:00.000+08:00"),
	))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:55:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 2)
	assert.Equal(t, taipeiAt(t, "2026-09-08T09:55:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), marketKCandles[1].OpenTime)
}

func TestFugleLeavesOutTheClosingAuctionThatEndsATaiwanSession(t *testing.T) {
	// The 13:30 closing auction is published as a candle but falls outside the session window, so it is dropped with other out-of-window candles.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330",
		fugleCandleJson("2026-09-08T13:28:00.000+08:00"),
		fugleCandleJson("2026-09-08T13:29:00.000+08:00"),
		fugleCandleJson("2026-09-08T13:30:00.000+08:00"),
	))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T13:31:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T13:28:00+08:00", "2026-09-08T13:29:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 2)
	assert.Equal(t, taipeiAt(t, "2026-09-08T13:28:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, taipeiAt(t, "2026-09-08T13:29:00+08:00").UTC(), marketKCandles[1].OpenTime,
		"當日最新一根的起始時間是 13:29，13:30 那一刻不構成一根")
}

func TestFugleReportsADayItHasNothingForAsNothing(t *testing.T) {
	// An empty stretch is not a failure.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330"))

	marketKCandles, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	assert.Empty(t, marketKCandles)
	assert.NotNil(t, marketKCandles)
}

func TestFugleReportsASourceThatWillNotAnswer(t *testing.T) {
	testCases := []struct {
		name   string
		handle http.HandlerFunc
	}{
		{
			name: "a refusal",
			handle: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusTooManyRequests)
			},
		},
		{
			name: "an answer that cannot be read",
			handle: func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(`{"data": not json`))
			},
		},
		{
			name: "a good answer with junk after it",
			handle: func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(fugleAnswerJson("2330") + `<html>an error page</html>`))
			},
		},
		{
			name: "a candle whose time cannot be read",
			handle: func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(fugleAnswerJson("2330", fugleCandleJson("yesterday"))))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(testCase.handle)
			t.Cleanup(server.Close)
			clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
			clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-08T10:07:00+08:00")).AnyTimes()
			fugleMarketDataProxy := marketdata.NewFugleMarketDataProxy(
				server.URL+"/intraday", server.URL+"/historical",
				"a-key", taipeiMarket(), clockProxy, requestTimeout, unpaced())

			_, fetchError := fugleMarketDataProxy.FetchKCandles(
				t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

			require.Error(t, fetchError)
		})
	}
}

func TestFugleAnswersWhetherASymbolExists(t *testing.T) {
	testCases := []struct {
		name           string
		status         int
		expectedExists bool
		expectedError  bool
	}{
		{name: "a listed stock", status: http.StatusOK, expectedExists: true},
		// 404 is an answer about the symbol.
		{name: "a code that is not listed", status: http.StatusNotFound, expectedExists: false},
		// A refusal says nothing about the symbol.
		{name: "a refusal", status: http.StatusTooManyRequests, expectedError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var seenRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, request *http.Request) {
					seenRequest = request
					writer.WriteHeader(testCase.status)
					_, _ = writer.Write([]byte(`{"symbol":"2330","name":"台積電"}`))
				}))
			t.Cleanup(server.Close)

			listing, lookupError := marketdata.NewFugleSymbolLookupProxy(
				server.URL+"/ticker", "a-key", requestTimeout, unpaced()).
				LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

			if testCase.expectedError {
				require.Error(t, lookupError)

				return
			}

			require.NoError(t, lookupError)
			assert.Equal(t, testCase.expectedExists, listing.IsListed)
			assert.Equal(t, "/ticker/2330", seenRequest.URL.Path)
			assert.Equal(t, "a-key", seenRequest.Header.Get("X-API-KEY"))
		})
	}
}

func TestFugleCarriesTheCompanyNameOutOfTheSameAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"symbol":"2330","name":"台積電","industry":"24"}`))
		}))
	t.Cleanup(server.Close)

	listing, lookupError := marketdata.NewFugleSymbolLookupProxy(
		server.URL+"/ticker", "a-key", requestTimeout, unpaced()).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	assert.Equal(t, "台積電", listing.DisplayName)
}

func TestFugleStillWatchesASymbolWhoseNameItCouldNotRead(t *testing.T) {
	// The code exists, so it may be watched even without a name.
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`not json at all`))
		}))
	t.Cleanup(server.Close)

	listing, lookupError := marketdata.NewFugleSymbolLookupProxy(
		server.URL+"/ticker", "a-key", requestTimeout, unpaced()).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	assert.Empty(t, listing.DisplayName)
}

func TestFugleReportsALookupItCannotReach(t *testing.T) {
	_, lookupError := marketdata.NewFugleSymbolLookupProxy(
		"http://127.0.0.1:1/ticker", "a-key", 50*time.Millisecond, unpaced()).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.Error(t, lookupError)
}

func TestFugleReportsAnAddressItCannotEvenAskAt(t *testing.T) {
	// A misconfigured address must fail rather than report an empty market.
	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-08T10:07:00+08:00")).AnyTimes()

	_, fetchError := marketdata.NewFugleMarketDataProxy(
		"http://\x7f/intraday", "http://\x7f/historical",
		"a-key", taipeiMarket(), clockProxy, requestTimeout, unpaced(),
	).FetchKCandles(t.Context(), fugleWindow(
		t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.Error(t, fetchError)
}

func TestFugleReportsALookupAddressItCannotEvenAskAt(t *testing.T) {
	_, lookupError := marketdata.NewFugleSymbolLookupProxy("http://\x7f/ticker", "a-key", requestTimeout, unpaced()).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.Error(t, lookupError)
}

func TestFugleNeverAsksAboutADayTheMarketCannotTradeOn(t *testing.T) {
	// Non-trading days must not cost a request, and the market domain decides which days those are.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330"))

	// Friday to Monday with "now" the following Tuesday, so every day goes to the historical address.
	_, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-15T20:00:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(
			t, "2026-09-11T09:00:00+08:00", "2026-09-14T13:29:00+08:00"))

	require.NoError(t, fetchError)

	askedDays := make([]string, 0)
	for _, request := range source.askedHistorical() {
		askedDays = append(askedDays, request.URL.Query().Get("from"))
	}
	assert.Equal(t, []string{"2026-09-11", "2026-09-14"}, askedDays,
		"週六與週日這個市場不可能有資料，不該為它們各打一次")
}
