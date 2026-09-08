package marketdata_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// taipei is the zone this source states its times in, fixed so that these rules hold
// on any machine whatever time zone database it ships with.
var taipei = time.FixedZone("Asia/Taipei", 8*60*60)

// taipeiAt is a moment said in Taipei time, which is how the requirements for this
// market are written.
func taipeiAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

// fugleCandleJson spells one candle the way this source does: a time with its own
// offset written into it, and figures as plain JSON numbers.
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

// fugleSourceUnderTest stands in for the two addresses this source answers at, and
// records what it was asked.
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
			// Keyed by the day asked about, with a day nobody set up answering with
			// nothing — a day standing in for another day would let a test pass on
			// candles it never actually asked for.
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
		"a-key", taipei, clockProxy, requestTimeout)
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
	// The source states its own offset; the system keeps every open time universal.
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, "574", marketKCandles[0].Open.String())
	assert.Equal(t, "576", marketKCandles[0].High.String())
	assert.Equal(t, "572", marketKCandles[0].Low.String())
	assert.Equal(t, "575", marketKCandles[0].Close.String())
	// Shares, exactly as reported. Converting to the lots a person reads on a screen
	// would be this system inventing a number nobody sent it.
	assert.Equal(t, "8450", marketKCandles[0].Volume.String())
}

func TestFugleLeavesTheFiguresItDoesNotPublishAbsent(t *testing.T) {
	// This venue publishes no turnover and no taker-buy breakdown on minute candles.
	// Absent is not zero: zero would say the market turned over nothing.
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
	// Today is only answered about at the intraday address. Asking the historical one
	// for today can come back empty long after the market has traded, which would read
	// as a quiet day — and, for a whole market, as a holiday.
	source := newFugleSourceUnderTest(t)
	source.answersWith("", fugleAnswerJson("2330"))

	_, fetchError := source.proxyAt(t, taipeiAt(t, "2026-09-08T10:07:00+08:00")).
		FetchKCandles(t.Context(), fugleWindow(t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, source.askedIntraday(), 1)
	assert.Empty(t, source.askedHistorical())
	assert.Equal(t, "1", source.askedIntraday()[0].URL.Query().Get("timeframe"))
	// Oldest first, said out loud: this source answers newest first unless told.
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
	// A window reaching back over a weekend touches two trading days, and the source
	// answers about one day at a time.
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
	// The source answers about whole days, so it will hand back candles from outside
	// the stretch that was actually wanted.
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

func TestFugleReportsADayItHasNothingForAsNothing(t *testing.T) {
	// Nothing is an answer. A market that did not trade in this stretch is not a
	// source that broke.
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
			// Ignored, a good answer followed by junk reads as a market with nothing to
			// report rather than as a source that cannot be read.
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
				"a-key", taipei, clockProxy, requestTimeout)

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
		// Not finding it is an answer about the symbol, not a failure to answer.
		{name: "a code that is not listed", status: http.StatusNotFound, expectedExists: false},
		// Being refused says nothing about the symbol, so it must not read as "no".
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
				server.URL+"/ticker", "a-key", requestTimeout).
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
	// The name is in the answer that proves the code real. Asking again for it would
	// be a second trip for something already on the desk.
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"symbol":"2330","name":"台積電","industry":"24"}`))
		}))
	t.Cleanup(server.Close)

	listing, lookupError := marketdata.NewFugleSymbolLookupProxy(
		server.URL+"/ticker", "a-key", requestTimeout).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	assert.Equal(t, "台積電", listing.DisplayName)
}

func TestFugleStillWatchesASymbolWhoseNameItCouldNotRead(t *testing.T) {
	// The source said the code exists, and that is the question that decides whether
	// somebody may watch it. A name is worth having and worth going without.
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`not json at all`))
		}))
	t.Cleanup(server.Close)

	listing, lookupError := marketdata.NewFugleSymbolLookupProxy(
		server.URL+"/ticker", "a-key", requestTimeout).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	assert.Empty(t, listing.DisplayName)
}

func TestFugleReportsALookupItCannotReach(t *testing.T) {
	_, lookupError := marketdata.NewFugleSymbolLookupProxy(
		"http://127.0.0.1:1/ticker", "a-key", 50*time.Millisecond).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.Error(t, lookupError)
}

func TestFugleReportsAnAddressItCannotEvenAskAt(t *testing.T) {
	// A misconfigured address is this system's fault rather than the market's, and it
	// has to say so out loud instead of reporting a market with nothing to say.
	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-08T10:07:00+08:00")).AnyTimes()

	_, fetchError := marketdata.NewFugleMarketDataProxy(
		"http://\x7f/intraday", "http://\x7f/historical",
		"a-key", taipei, clockProxy, requestTimeout,
	).FetchKCandles(t.Context(), fugleWindow(
		t, "2026-09-08T09:40:00+08:00", "2026-09-08T10:00:00+08:00"))

	require.Error(t, fetchError)
}

func TestFugleReportsALookupAddressItCannotEvenAskAt(t *testing.T) {
	_, lookupError := marketdata.NewFugleSymbolLookupProxy("http://\x7f/ticker", "a-key", requestTimeout).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.Error(t, lookupError)
}
