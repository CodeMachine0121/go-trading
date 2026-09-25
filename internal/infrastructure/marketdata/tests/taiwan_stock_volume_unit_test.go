package marketdata_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Both paths read the same answer, so any difference is something this system introduced; expected values are the raw reported figures with no arithmetic.
func TestTheLiveFollowAndTheStoredHistoryReportAMinuteIdentically(t *testing.T) {
	const reportedVolume = "344"
	const reportedClose = "112.95"

	answer := fmt.Sprintf(
		`{"symbol":"0050","data":[{"date":"2026-09-23T09:30:00.000+08:00",`+
			`"open":112.75,"high":113,"low":112.5,"close":%s,"volume":%s,"average":112.9}]}`,
		reportedClose, reportedVolume)

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, _ *http.Request,
	) {
		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)

	historyKCandle := fetchStoredMinute(t, server.URL)
	liveKCandle := followLiveMinute(t, server.URL)

	assert.Equal(t, historyKCandle.Volume.String(), liveKCandle.Volume.String(),
		"the same minute must carry the same volume whichever path reported it")
	assert.Equal(t, historyKCandle.Close.String(), liveKCandle.Close.String(),
		"the same minute must carry the same close whichever path reported it")
	assert.True(t, historyKCandle.OpenTime.Equal(liveKCandle.OpenTime),
		"the same minute must be filed under the same moment either way")

	// Asserted against the answer, not each other, so both agreeing on an invented figure still fails.
	assert.Equal(t, reportedVolume, liveKCandle.Volume.String())
	assert.Equal(t, reportedClose, liveKCandle.Close.String())
}

func fetchStoredMinute(t *testing.T, sourceUrl string) vo.MarketKCandleVo {
	t.Helper()

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-23T09:35:00+08:00")).AnyTimes()

	marketKCandles, fetchError := marketdata.NewFugleMarketDataProxy(
		sourceUrl+"/intraday", sourceUrl+"/historical", "a-key", taipeiMarket(),
		clockProxy, 2*time.Second, marketdata.NewRequestPacer(0),
	).FetchKCandles(t.Context(), vo.NewKCandleFetchWindowVo(
		"0050", vo.MarketTaiwanStock,
		taipeiAt(t, "2026-09-23T09:30:00+08:00"),
		taipeiAt(t, "2026-09-23T09:31:00+08:00")))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)

	return marketKCandles[0]
}

func followLiveMinute(t *testing.T, sourceUrl string) vo.LiveKCandleVo {
	t.Helper()

	executionContext, stopFollowing := contextWithCancel(t)

	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		sourceUrl, "a-key", time.Millisecond, 0, time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"0050"}))
	require.NoError(t, followError)
	defer stopFollowing()

	return nextKCandle(t, liveKCandles)
}
