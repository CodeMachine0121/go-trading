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

// The two paths a Taiwan candle can reach the system by are read side by side here,
// from one answer, on purpose.
//
// This feature has now been wrong twice about how the live figures relate to the
// stored ones — once about the unit, once about which field carried a price — and
// both times every test stayed green, because every test asked one path in isolation
// about data I had written myself. A live update only means anything next to the
// history it lands beside, so that relationship is what this pins.
//
// Both paths now ask the same venue for the same shape, so a difference between them
// could only be something this system did on the way in. There is deliberately no
// arithmetic anywhere in the expected values below: the answer states 344, and both
// paths must say 344.
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

	// Said against the answer rather than against each other, so that both paths
	// agreeing on something invented would still fail.
	assert.Equal(t, reportedVolume, liveKCandle.Volume.String())
	assert.Equal(t, reportedClose, liveKCandle.Close.String())
}

// fetchStoredMinute takes the path the scheduled round takes.
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

// followLiveMinute takes the path a viewer's chart takes.
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
