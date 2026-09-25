package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statisticAt(hour int, minute int) time.Time {
	return time.Date(2026, 9, 23, hour, minute, 0, 0, time.UTC)
}

func openInterestJson(statisticTime time.Time, openInterest string) string {
	return fmt.Sprintf(`{"symbol":"BTCUSDT","sumOpenInterest":"%s","sumOpenInterestValue":"9262820059.48","CMCCirculatingSupply":"1","timestamp":%d}`,
		openInterest, statisticTime.UnixMilli())
}

func splitJson(statisticTime time.Time, longShare string, shortShare string, ratio string) string {
	return fmt.Sprintf(`{"symbol":"BTCUSDT","longAccount":"%s","shortAccount":"%s","longShortRatio":"%s","timestamp":%d}`,
		longShare, shortShare, ratio, statisticTime.UnixMilli())
}

type statisticsVenue struct {
	baseUrl   string
	lock      *sync.Mutex
	questions *[]url.URL
}

func servedByStatisticsVenue(t *testing.T, bodies map[string]string) statisticsVenue {
	t.Helper()

	venue := statisticsVenue{lock: &sync.Mutex{}, questions: &[]url.URL{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		venue.lock.Lock()
		*venue.questions = append(*venue.questions, *request.URL)
		venue.lock.Unlock()

		body, known := bodies[request.URL.Path]
		if !known {
			_, _ = writer.Write([]byte("[]"))

			return
		}
		if body == "refuse" {
			writer.WriteHeader(http.StatusBadRequest)

			return
		}
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	venue.baseUrl = server.URL

	return venue
}

func (venue statisticsVenue) asked() []url.URL {
	venue.lock.Lock()
	defer venue.lock.Unlock()

	return append([]url.URL{}, *venue.questions...)
}

func (venue statisticsVenue) proxy() *marketdata.BinanceContractPositionStatisticProxy {
	return marketdata.NewBinanceContractPositionStatisticProxy(venue.baseUrl, requestTimeout, unpaced())
}

func TestPositionStatisticProxyAlignsTheThreeAnswersOnTheStatisticTime(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{
		"/openInterestHist": "[" + openInterestJson(statisticAt(9, 5), "106479.865") + "," +
			openInterestJson(statisticAt(9, 10), "106500") + "]",
		"/globalLongShortAccountRatio": "[" + splitJson(statisticAt(9, 5), "0.47", "0.53", "0.89") + "," +
			splitJson(statisticAt(9, 10), "0.5", "0.5", "1") + "]",
		"/topLongShortPositionRatio": "[" + splitJson(statisticAt(9, 5), "0.6688", "0.3312", "2.0189") + "]",
	})

	statistics, fetchError := venue.proxy().FetchPositionStatistics(
		t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	require.NoError(t, fetchError)
	require.Len(t, statistics, 2)
	first := statistics[0]
	assert.Equal(t, "BTCUSDT", first.Symbol)
	assert.Equal(t, statisticAt(9, 5), first.StatisticTime)
	assert.True(t, decimal.RequireFromString("106479.865").Equal(first.OpenInterest))
	assert.True(t, decimal.RequireFromString("9262820059.48").Equal(first.OpenInterestValue))
	assert.True(t, decimal.RequireFromString("0.47").Equal(first.AccountLongShare.Decimal))
	assert.True(t, decimal.RequireFromString("0.53").Equal(first.AccountShortShare.Decimal))
	assert.True(t, decimal.RequireFromString("0.89").Equal(first.AccountLongShortRatio.Decimal))
	assert.True(t, decimal.RequireFromString("0.6688").Equal(first.TopTraderPositionLongShare.Decimal))
	assert.True(t, decimal.RequireFromString("0.3312").Equal(first.TopTraderPositionShortShare.Decimal))
	assert.True(t, decimal.RequireFromString("2.0189").Equal(first.TopTraderPositionLongShortRatio.Decimal))
	second := statistics[1]
	assert.True(t, second.AccountLongShare.Valid)
	assert.False(t, second.TopTraderPositionLongShare.Valid)
	assert.False(t, second.TopTraderPositionShortShare.Valid)
	assert.False(t, second.TopTraderPositionLongShortRatio.Valid)
}

func TestPositionStatisticProxyNamesBothEndsAndTheFiveMinutePeriod(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{
		"/openInterestHist": "[" + openInterestJson(statisticAt(9, 5), "1") + "]",
	})

	_, fetchError := venue.proxy().FetchPositionStatistics(
		t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	require.NoError(t, fetchError)
	questions := venue.asked()
	require.Len(t, questions, 3)
	for _, question := range questions {
		assert.Equal(t, "BTCUSDT", question.Query().Get("symbol"))
		assert.Equal(t, "5m", question.Query().Get("period"))
		assert.Equal(t, "500", question.Query().Get("limit"))
		assert.Equal(t, fmt.Sprint(statisticAt(9, 5).UnixMilli()), question.Query().Get("startTime"))
		assert.Equal(t, fmt.Sprint(statisticAt(9, 10).UnixMilli()), question.Query().Get("endTime"))
	}
}

func TestPositionStatisticProxyWalksAWindowLongerThanOnePage(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{})
	windowStart := statisticAt(0, 0)
	// 501 grid moments: one page of 500 and one of 1.
	windowEnd := windowStart.Add(500 * 5 * time.Minute)

	_, fetchError := venue.proxy().FetchPositionStatistics(t.Context(), "BTCUSDT", windowStart, windowEnd)

	require.NoError(t, fetchError)
	questions := venue.asked()
	require.Len(t, questions, 2)
	assert.Equal(t, fmt.Sprint(windowStart.UnixMilli()), questions[0].Query().Get("startTime"))
	assert.Equal(t, fmt.Sprint(windowStart.Add(499*5*time.Minute).UnixMilli()), questions[0].Query().Get("endTime"))
	assert.Equal(t, fmt.Sprint(windowEnd.UnixMilli()), questions[1].Query().Get("startTime"))
	assert.Equal(t, fmt.Sprint(windowEnd.UnixMilli()), questions[1].Query().Get("endTime"))
}

func TestPositionStatisticProxyAsksNoSplitWhenNothingWasOpen(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{"/openInterestHist": "[]"})

	statistics, fetchError := venue.proxy().FetchPositionStatistics(
		t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	require.NoError(t, fetchError)
	assert.Empty(t, statistics)
	require.Len(t, venue.asked(), 1)
	assert.Equal(t, "/openInterestHist", venue.asked()[0].Path)
}

func TestPositionStatisticProxyKeepsOnlyTheMomentsInsideTheWindow(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{
		"/openInterestHist": "[" + openInterestJson(statisticAt(9, 0), "1") + "," +
			openInterestJson(statisticAt(9, 5), "2") + "," + openInterestJson(statisticAt(9, 15), "3") + "]",
	})

	statistics, fetchError := venue.proxy().FetchPositionStatistics(
		t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	require.NoError(t, fetchError)
	require.Len(t, statistics, 1)
	assert.Equal(t, statisticAt(9, 5), statistics[0].StatisticTime)
}

func TestPositionStatisticProxyFailsWhenAnyOfTheThreeCannotBeRead(t *testing.T) {
	aMoment := "[" + openInterestJson(statisticAt(9, 5), "1") + "]"
	testCases := []struct {
		name   string
		bodies map[string]string
	}{
		{name: "持倉量被拒絕", bodies: map[string]string{"/openInterestHist": "refuse"}},
		{name: "多空人數比被拒絕", bodies: map[string]string{"/openInterestHist": aMoment, "/globalLongShortAccountRatio": "refuse"}},
		{name: "大戶多空持倉比被拒絕", bodies: map[string]string{"/openInterestHist": aMoment, "/topLongShortPositionRatio": "refuse"}},
		{name: "持倉量讀不懂", bodies: map[string]string{"/openInterestHist": `{"code":-1130}`}},
		{name: "持倉量不是數字", bodies: map[string]string{"/openInterestHist": "[" + openInterestJson(statisticAt(9, 5), "nope") + "]"}},
		{name: "持倉價值不是數字", bodies: map[string]string{"/openInterestHist": strings.Replace(aMoment, "9262820059.48", "nope", 1)}},
		{name: "多空人數比讀不懂", bodies: map[string]string{"/openInterestHist": aMoment, "/globalLongShortAccountRatio": `{}`}},
		{name: "多空人數比不是數字", bodies: map[string]string{"/openInterestHist": aMoment,
			"/globalLongShortAccountRatio": "[" + splitJson(statisticAt(9, 5), "nope", "0.5", "1") + "]"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			venue := servedByStatisticsVenue(t, testCase.bodies)

			statistics, fetchError := venue.proxy().FetchPositionStatistics(
				t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

			assert.Error(t, fetchError)
			assert.Nil(t, statistics)
		})
	}
}

func TestPositionStatisticProxySaysSoWhenTheVenueCannotBeReached(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1:1", controlCharacterUrl} {
		positionStatisticProxy := marketdata.NewBinanceContractPositionStatisticProxy(address, requestTimeout, unpaced())

		_, fetchError := positionStatisticProxy.FetchPositionStatistics(
			t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

		assert.ErrorContains(t, fetchError, "reach contract position statistic source")
	}
}

func TestPositionStatisticProxyGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	venue := servedByStatisticsVenue(t, map[string]string{})
	positionStatisticProxy := marketdata.NewBinanceContractPositionStatisticProxy(
		venue.baseUrl, requestTimeout, marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, fetchError := positionStatisticProxy.FetchPositionStatistics(
		abandonedContext, "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	assert.ErrorIs(t, fetchError, context.Canceled)
}

func TestPositionStatisticProxySaysSoWhenTheAnswerIsCutOff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "100")
		_, _ = writer.Write([]byte("["))
	}))
	t.Cleanup(server.Close)
	positionStatisticProxy := marketdata.NewBinanceContractPositionStatisticProxy(server.URL, requestTimeout, unpaced())

	_, fetchError := positionStatisticProxy.FetchPositionStatistics(
		t.Context(), "BTCUSDT", statisticAt(9, 5), statisticAt(9, 10))

	assert.ErrorContains(t, fetchError, "read contract position statistic answer")
}
