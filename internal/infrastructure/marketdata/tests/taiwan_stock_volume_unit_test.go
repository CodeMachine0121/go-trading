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

// The two Taiwan sources are read side by side here on purpose.
//
// Each one alone is already pinned: history carries volume across exactly as reported,
// live multiplies lots into shares. Both can be right on their own and still disagree
// with each other, because the venues count in different units — and nothing would
// ever raise an error about it. The same stock's chart would simply show a thousandfold
// step wherever the history ends and the live updates begin, and every judgment that
// reads volume would stop meaning anything without saying so.
//
// This is the only test that compares them, which is why it is worth its own file.
func TestBothTaiwanSourcesReportTheSameMinuteInTheSameUnit(t *testing.T) {
	const tradedLots = 8450
	const tradedShares = tradedLots * 1000

	historyVolume := fetchHistoryVolume(t, tradedShares)
	liveVolume := followLiveVolume(t, tradedLots)

	assert.Equal(t, historyVolume, liveVolume,
		"the same minute must carry the same number of shares whichever source reported it")
}

// fetchHistoryVolume asks the history source for one minute in which the given number
// of shares changed hands, and reports what the system made of it.
func fetchHistoryVolume(t *testing.T, tradedShares int) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, _ *http.Request,
	) {
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`{"symbol":"2330","data":[{"date":"2026-09-22T10:00:00.000+08:00",`+
				`"open":2500,"high":2500,"low":2500,"close":2500,"volume":%d}]}`,
			tradedShares)))
	}))
	t.Cleanup(server.Close)

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(taipeiAt(t, "2026-09-22T10:07:00+08:00")).AnyTimes()

	marketKCandles, fetchError := marketdata.NewFugleMarketDataProxy(
		server.URL+"/intraday", server.URL+"/historical", "a-key", taipeiMarket(),
		clockProxy, 2*time.Second, marketdata.NewRequestPacer(0),
	).FetchKCandles(t.Context(), vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock,
		taipeiAt(t, "2026-09-22T10:00:00+08:00"),
		taipeiAt(t, "2026-09-22T10:01:00+08:00")))

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)

	return marketKCandles[0].Volume.String()
}

// followLiveVolume follows the live source through one minute in which the given
// number of lots changed hands, and reports what the system made of it.
func followLiveVolume(t *testing.T, tradedLots int) string {
	t.Helper()

	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "0")},
		[]twseQuote{quoteAt("10:00:45", "2500", fmt.Sprint(tradedLots))},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	return nextKCandle(t, liveKCandles).Volume.String()
}
