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
// They count in the same unit — measured, not assumed: on 2026-09-22 the history
// source's minute volumes for 2330 summed to 18876 and the exchange's running total
// closed at the same 18876, to the unit. Neither converts, so the same trading
// reported by both must arrive as the same number.
//
// Each source alone cannot show this. Both would still look right on their own with
// a conversion quietly applied to one of them, and the only visible symptom would be
// a thousandfold step in a chart exactly where the stored history ends and the live
// updates begin — with every judgment that reads volume meaningless from there on,
// and nothing raising an error about it.
//
// So the figure below is deliberately the *same* on both sides of the test. Scale
// either source and this fails.
func TestBothTaiwanSourcesReportTheSameMinuteInTheSameUnit(t *testing.T) {
	const tradedVolume = 8450

	assert.Equal(t,
		fetchHistoryVolume(t, tradedVolume),
		followLiveVolume(t, tradedVolume),
		"the same trading must carry the same number whichever source reported it")
}

// fetchHistoryVolume asks the history source for one minute carrying the given
// volume, and reports what the system made of it.
func fetchHistoryVolume(t *testing.T, tradedVolume int) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, _ *http.Request,
	) {
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`{"symbol":"2330","data":[{"date":"2026-09-22T10:00:00.000+08:00",`+
				`"open":2500,"high":2500,"low":2500,"close":2500,"volume":%d}]}`,
			tradedVolume)))
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

// followLiveVolume follows the live source through one minute carrying the given
// volume, and reports what the system made of it.
func followLiveVolume(t *testing.T, tradedVolume int) string {
	t.Helper()

	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "0")},
		[]twseQuote{quoteAt("10:00:45", "2500", fmt.Sprint(tradedVolume))},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	return nextKCandle(t, liveKCandles).Volume.String()
}
