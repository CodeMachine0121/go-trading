package marketdata_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// fubonSourceUnderTest stands in for the two addresses this source answers at — the
// list of contracts it trades, and one board's candles at a time — and records what
// it was asked, so a test can say which contract was fetched and under which board.
type fubonSourceUnderTest struct {
	server *httptest.Server

	mutex            sync.Mutex
	productsAnswer   string
	productsStatus   int
	candlesBySession map[string]string
	candlesStatus    int
	candleRequests   []*http.Request
}

func newFubonSourceUnderTest(t *testing.T) *fubonSourceUnderTest {
	t.Helper()

	source := &fubonSourceUnderTest{
		productsStatus:   http.StatusOK,
		candlesStatus:    http.StatusOK,
		candlesBySession: make(map[string]string),
	}
	source.server = httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			source.mutex.Lock()
			defer source.mutex.Unlock()

			if strings.HasPrefix(request.URL.Path, "/products") {
				writer.WriteHeader(source.productsStatus)
				_, _ = writer.Write([]byte(source.productsAnswer))

				return
			}

			source.candleRequests = append(source.candleRequests, request)
			writer.WriteHeader(source.candlesStatus)
			_, _ = writer.Write([]byte(source.candlesBySession[request.URL.Query().Get("session")]))
		}))
	t.Cleanup(source.server.Close)

	return source
}

func (source *fubonSourceUnderTest) lists(contracts ...string) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	listed := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		listed = append(listed,
			fmt.Sprintf(`{"symbol":"%s","name":"臺股期貨%s"}`, contract, contract))
	}
	source.productsAnswer = `{"data":[` + strings.Join(listed, ",") + `]}`
}

func (source *fubonSourceUnderTest) answersBoard(session string, answer string) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.candlesBySession[session] = answer
}

func (source *fubonSourceUnderTest) fetchedContracts() []string {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	contracts := make([]string, 0, len(source.candleRequests))
	for _, request := range source.candleRequests {
		contracts = append(contracts, strings.TrimPrefix(request.URL.Path, "/candles/"))
	}

	return contracts
}

func (source *fubonSourceUnderTest) askedBoards() []string {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	boards := make([]string, 0, len(source.candleRequests))
	for _, request := range source.candleRequests {
		boards = append(boards, request.URL.Query().Get("session"))
	}

	return boards
}

// futuresProxyUnderTest is the proxy pointed at the stand-in source, with the clock
// it reads the current year from — the year is what turns the single digit this venue
// writes years as into a year.
func futuresProxyUnderTest(
	t *testing.T, source *fubonSourceUnderTest, now string,
) *marketdata.FubonMarketDataProxy {
	t.Helper()

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, now)).AnyTimes()

	return marketdata.NewFubonMarketDataProxy(
		source.server.URL+"/products", source.server.URL+"/candles",
		"an-api-key", clockProxy, time.Second)
}

func futuresWindow(t *testing.T, startTime string, endTime string) vo.KCandleFetchWindowVo {
	t.Helper()

	return vo.NewKCandleFetchWindowVo(
		"TXF", vo.MarketTaiwanFutures, taipeiAt(t, startTime), taipeiAt(t, endTime))
}

func TestTheFuturesSourceIsAskedForTheContractThatExpiresSoonest(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	// Listed out of order on purpose, and with a contract of another underlying
	// mixed in: the answer is about which of this code's contracts goes first.
	source.lists("TXFL6", "TXFJ6", "MXFI6", "TXFI6")
	source.answersBoard("REGULAR", fugleAnswerJson("TXFI6",
		fugleCandleJson("2026-09-10T10:06:00.000+08:00")))
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFI6"))

	_, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T10:07:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T10:00:00+08:00", "2026-09-10T10:07:00+08:00"))

	require.NoError(t, fetchError)
	// September is I, and 2026 ends in a 6.
	assert.Equal(t, []string{"TXFI6", "TXFI6"}, source.fetchedContracts())
}

func TestTheFuturesSourceRollsWhenTheVenueStopsListingTheExpiringContract(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	// Settlement has happened, so the venue no longer lists September.
	source.lists("TXFJ6", "TXFL6")
	source.answersBoard("REGULAR", fugleAnswerJson("TXFJ6"))
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFJ6",
		fugleCandleJson("2026-09-16T22:29:00.000+08:00")))

	marketKCandles, fetchError := futuresProxyUnderTest(t, source, "2026-09-16T22:30:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-16T22:00:00+08:00", "2026-09-16T22:30:00+08:00"))

	require.NoError(t, fetchError)
	assert.Equal(t, []string{"TXFJ6", "TXFJ6"}, source.fetchedContracts())
	// The candle still arrives under the standing code, which is what keeps one
	// continuous line across the roll.
	require.Len(t, marketKCandles, 1)
	assert.Equal(t, "TXF", marketKCandles[0].Symbol)
}

func TestTheFuturesSourceIsAskedAboutBothBoards(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.lists("TXFI6")
	// The evening board's candles start on the calendar day before the day board's,
	// and they arrive in a separate answer.
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFI6",
		fugleCandleJson("2026-09-10T02:00:00.000+08:00")))
	source.answersBoard("REGULAR", fugleAnswerJson("TXFI6",
		fugleCandleJson("2026-09-10T10:06:00.000+08:00")))

	marketKCandles, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T10:07:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T00:00:00+08:00", "2026-09-10T10:07:00+08:00"))

	require.NoError(t, fetchError)
	assert.ElementsMatch(t, []string{"REGULAR", "AFTERHOURS"}, source.askedBoards())
	// One sequence, oldest first, whichever answer each candle came in.
	require.Len(t, marketKCandles, 2)
	assert.Equal(t, taipeiAt(t, "2026-09-10T02:00:00+08:00").UTC(), marketKCandles[0].OpenTime)
	assert.Equal(t, taipeiAt(t, "2026-09-10T10:06:00+08:00").UTC(), marketKCandles[1].OpenTime)
}

func TestFuturesCandlesOutsideTheWindowAreLeftOut(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.lists("TXFI6")
	source.answersBoard("REGULAR", fugleAnswerJson("TXFI6",
		fugleCandleJson("2026-09-10T09:30:00.000+08:00"),
		fugleCandleJson("2026-09-10T10:06:00.000+08:00")))
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFI6"))

	marketKCandles, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T10:07:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T10:00:00+08:00", "2026-09-10T10:07:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	assert.Equal(t, taipeiAt(t, "2026-09-10T10:06:00+08:00").UTC(), marketKCandles[0].OpenTime)
}

func TestFuturesCandlesCarryNoFiguresThisVenueDoesNotPublish(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.lists("TXFI6")
	source.answersBoard("REGULAR", fugleAnswerJson("TXFI6",
		fugleCandleJson("2026-09-10T10:06:00.000+08:00")))
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFI6"))

	marketKCandles, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T10:07:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T10:00:00+08:00", "2026-09-10T10:07:00+08:00"))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	// Absent rather than zero: zero is a figure this venue could legitimately report.
	assert.False(t, marketKCandles[0].QuoteVolume.Valid)
	assert.False(t, marketKCandles[0].TakerBuyBaseVolume.Valid)
	assert.False(t, marketKCandles[0].TakerBuyQuoteVolume.Valid)
	// Volume comes across exactly as reported: this venue counts in lots and turning
	// them into anything else would be this system inventing a number.
	assert.Equal(t, "8450", marketKCandles[0].Volume.String())
}

func TestAFuturesSourceThatListsNoContractIsAFailureWhenFetching(t *testing.T) {
	// Nobody can say what "the futures" is right now. Answering with no candles would
	// store nothing and report nothing wrong, on a market that may have traded all
	// evening.
	source := newFubonSourceUnderTest(t)
	source.lists("MXFI6")

	_, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T22:30:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T22:00:00+08:00", "2026-09-10T22:30:00+08:00"))

	require.Error(t, fetchError)
	assert.Empty(t, source.fetchedContracts())
}

func TestAFuturesSourceThatWillNotAnswerIsAFailure(t *testing.T) {
	testCases := []struct {
		name           string
		productsStatus int
		candlesStatus  int
	}{
		{name: "the contract list is refused", productsStatus: http.StatusUnauthorized, candlesStatus: http.StatusOK},
		{name: "the candles are refused", productsStatus: http.StatusOK, candlesStatus: http.StatusTooManyRequests},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newFubonSourceUnderTest(t)
			source.lists("TXFI6")
			source.productsStatus = testCase.productsStatus
			source.candlesStatus = testCase.candlesStatus
			source.answersBoard("REGULAR", fugleAnswerJson("TXFI6"))

			_, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T22:30:00+08:00").
				FetchKCandles(t.Context(), futuresWindow(
					t, "2026-09-10T22:00:00+08:00", "2026-09-10T22:30:00+08:00"))

			require.Error(t, fetchError)
		})
	}
}

func TestAFuturesContractListThatCannotBeReadIsAFailure(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.productsAnswer = "not json at all"

	_, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T22:30:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2026-09-10T22:00:00+08:00", "2026-09-10T22:30:00+08:00"))

	require.Error(t, fetchError)
}

func TestFuturesCandlesThatCannotBeReadAreAFailure(t *testing.T) {
	testCases := []struct {
		name          string
		regularAnswer string
	}{
		{name: "the answer is not readable", regularAnswer: "not json at all"},
		{
			name: "a candle states a time nothing can read",
			regularAnswer: `{"symbol":"TXFI6","data":[` +
				`{"date":"the tenth of september","open":1,"high":1,"low":1,"close":1,"volume":1}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newFubonSourceUnderTest(t)
			source.lists("TXFI6")
			source.answersBoard("REGULAR", testCase.regularAnswer)

			_, fetchError := futuresProxyUnderTest(t, source, "2026-09-10T22:30:00+08:00").
				FetchKCandles(t.Context(), futuresWindow(
					t, "2026-09-10T22:00:00+08:00", "2026-09-10T22:30:00+08:00"))

			require.Error(t, fetchError)
		})
	}
}

func TestTheYearDigitIsReadAgainstTheYearItIsAskedIn(t *testing.T) {
	// The venue writes a year as one digit, so a digit only names a year relative to
	// now. Across the turn of a decade, comparing digits alone would put 2030's
	// contract before 2029's — and the roll would jump a year early.
	source := newFubonSourceUnderTest(t)
	source.lists("TXFA0", "TXFL9")
	source.answersBoard("REGULAR", fugleAnswerJson("TXFL9"))
	source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFL9"))

	_, fetchError := futuresProxyUnderTest(t, source, "2029-12-10T10:07:00+08:00").
		FetchKCandles(t.Context(), futuresWindow(
			t, "2029-12-10T10:00:00+08:00", "2029-12-10T10:07:00+08:00"))

	require.NoError(t, fetchError)
	// December 2029 comes before January 2030.
	assert.Equal(t, []string{"TXFL9", "TXFL9"}, source.fetchedContracts())
}

func TestTheFuturesLookUpAnswersWithTheContractTheVenueLists(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.lists("TXFI6", "TXFJ6")

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-10T10:07:00+08:00")).AnyTimes()
	listing, lookUpError := marketdata.NewFubonSymbolLookupProxy(
		source.server.URL+"/products", "an-api-key", clockProxy, time.Second).
		LookUpSymbol(t.Context(), vo.MarketTaiwanFutures, "TXF")

	require.NoError(t, lookUpError)
	assert.True(t, listing.IsListed)
	assert.Equal(t, "臺股期貨TXFI6", listing.DisplayName)
}

func TestTheFuturesLookUpSaysNotListedRatherThanFailingForACodeThisVenueDoesNotTrade(t *testing.T) {
	// "No such code" is an answer about the code — the typo this check exists to
	// catch — while an unreachable source says nothing about the code at all.
	source := newFubonSourceUnderTest(t)
	source.lists("TXFI6")

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-10T10:07:00+08:00")).AnyTimes()
	listing, lookUpError := marketdata.NewFubonSymbolLookupProxy(
		source.server.URL+"/products", "an-api-key", clockProxy, time.Second).
		LookUpSymbol(t.Context(), vo.MarketTaiwanFutures, "NOSUCH")

	require.NoError(t, lookUpError)
	assert.False(t, listing.IsListed)
}

func TestTheFuturesLookUpFailsWhenTheSourceCannotBeAsked(t *testing.T) {
	source := newFubonSourceUnderTest(t)
	source.productsStatus = http.StatusUnauthorized

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-10T10:07:00+08:00")).AnyTimes()
	_, lookUpError := marketdata.NewFubonSymbolLookupProxy(
		source.server.URL+"/products", "an-api-key", clockProxy, time.Second).
		LookUpSymbol(t.Context(), vo.MarketTaiwanFutures, "TXF")

	require.Error(t, lookUpError)
}

func TestOnlyASymbolShapedLikeAContractOfTheCodeIsConsidered(t *testing.T) {
	testCases := []struct {
		name     string
		listed   []string
		expected string
	}{
		// The venue writes a delivery month as a letter and a year as one digit, so a
		// symbol carrying anything else after the code is not one of its contracts.
		{
			name:   "a longer suffix is not a contract of this code",
			listed: []string{"TXFI66", "TXFJ6"}, expected: "TXFJ6",
		},
		{
			name:   "a letter that names no month is not a contract",
			listed: []string{"TXFZ6", "TXFJ6"}, expected: "TXFJ6",
		},
		{
			name:   "a year that is not a digit is not a contract",
			listed: []string{"TXFIX", "TXFJ6"}, expected: "TXFJ6",
		},
		// A listing left over from last year still orders before this year's, which is
		// what stops a stale row being read as a contract a decade out.
		{
			name:   "a contract from the year before still orders first",
			listed: []string{"TXFC0", "TXFL9"}, expected: "TXFL9",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newFubonSourceUnderTest(t)
			source.lists(testCase.listed...)
			source.answersBoard("REGULAR", fugleAnswerJson(testCase.expected))
			source.answersBoard("AFTERHOURS", fugleAnswerJson(testCase.expected))

			_, fetchError := futuresProxyUnderTest(t, source, "2030-03-10T10:07:00+08:00").
				FetchKCandles(t.Context(), futuresWindow(
					t, "2030-03-10T10:00:00+08:00", "2030-03-10T10:07:00+08:00"))

			require.NoError(t, fetchError)
			assert.Equal(t,
				[]string{testCase.expected, testCase.expected}, source.fetchedContracts())
		})
	}
}

func TestAFuturesSourceThatCannotBeReachedAtAllIsAFailure(t *testing.T) {
	testCases := []struct {
		name            string
		productsUrl     string
		candlesUrl      string
		servesContracts bool
	}{
		// Nothing is listening, which is a different thing from a source that answered.
		{name: "the contract list refuses the connection", productsUrl: "http://127.0.0.1:1/products"},
		{
			name:       "the candles refuse the connection",
			candlesUrl: "http://127.0.0.1:1/candles", servesContracts: true,
		},
		// An address nothing can even be asked at.
		{name: "the contract list address is unusable", productsUrl: "http://\x7f/products"},
		{
			name:       "the candles address is unusable",
			candlesUrl: "http://\x7f/candles", servesContracts: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newFubonSourceUnderTest(t)
			source.lists("TXFI6")
			source.answersBoard("REGULAR", fugleAnswerJson("TXFI6"))
			source.answersBoard("AFTERHOURS", fugleAnswerJson("TXFI6"))

			productsUrl := source.server.URL + "/products"
			if !testCase.servesContracts {
				productsUrl = testCase.productsUrl
			}
			candlesUrl := source.server.URL + "/candles"
			if testCase.candlesUrl != "" {
				candlesUrl = testCase.candlesUrl
			}

			clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
			clockProxy.EXPECT().Now().
				Return(taipeiAt(t, "2026-09-10T22:30:00+08:00")).AnyTimes()

			_, fetchError := marketdata.NewFubonMarketDataProxy(
				productsUrl, candlesUrl, "an-api-key", clockProxy, time.Second).
				FetchKCandles(t.Context(), futuresWindow(
					t, "2026-09-10T22:00:00+08:00", "2026-09-10T22:30:00+08:00"))

			require.Error(t, fetchError)
		})
	}
}
