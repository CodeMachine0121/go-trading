package marketdata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneMessageFeed stands in for the market source: it says one thing and hangs up.
func oneMessageFeed(t *testing.T, message string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter, request *http.Request,
	) {
		connection, acceptError := websocket.Accept(responseWriter, request, nil)
		if acceptError != nil {
			return
		}
		defer func() { _ = connection.CloseNow() }()

		_ = connection.Write(request.Context(), websocket.MessageText, []byte(message))
		time.Sleep(50 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	return server
}

func streamUrlOf(server *httptest.Server) string {
	return "ws" + server.URL[len("http"):]
}

// Following asks the source for five-minute candles, spelled the way it wants them.
// Asking for the wrong length would quietly deliver candles of another size, and
// nothing downstream could tell.
func TestTheFeedIsOpenedForFiveMinuteCandlesOfThatSymbol(t *testing.T) {
	askedFor := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter, request *http.Request,
	) {
		askedFor <- request.URL.Path
		connection, acceptError := websocket.Accept(responseWriter, request, nil)
		if acceptError != nil {
			return
		}
		_ = connection.CloseNow()
	}))
	t.Cleanup(server.Close)

	_, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), "BTCUSDT")
	require.NoError(t, followError)

	assert.Equal(t, "/btcusdt@kline_5m", <-askedFor)
}

// The live message names its fields in one or two letters. Translating them is this
// layer's whole job, and getting one wrong would put a price in a volume.
func TestALiveMessageIsNormalizedIntoOneCandle(t *testing.T) {
	server := oneMessageFeed(t, `{"k":{
		"t":1788404700000,"s":"BTCUSDT","o":"100.5","c":"118.25","h":"120","l":"90",
		"v":"12.5","q":"1400.75","V":"7.25","Q":"800.5","x":true}}`)

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), "BTCUSDT")
	require.NoError(t, followError)

	liveKCandle := <-liveKCandles

	assert.Equal(t, "BTCUSDT", liveKCandle.Symbol)
	assert.Equal(t, time.UnixMilli(1788404700000).UTC(), liveKCandle.OpenTime)
	assert.Equal(t, "100.5", liveKCandle.Open.String())
	assert.Equal(t, "120", liveKCandle.High.String())
	assert.Equal(t, "90", liveKCandle.Low.String())
	assert.Equal(t, "118.25", liveKCandle.Close.String())
	assert.Equal(t, "12.5", liveKCandle.Volume.String())
	assert.Equal(t, "1400.75", liveKCandle.QuoteVolume.String())
	assert.Equal(t, "7.25", liveKCandle.TakerBuyBaseVolume.String())
	assert.Equal(t, "800.5", liveKCandle.TakerBuyQuoteVolume.String())
	assert.True(t, liveKCandle.Closed, "來源說這一根走完了")
}

// A candle still running is the ordinary case, and it must be reported as such —
// everything downstream decides what may be stored from this one flag.
func TestACandleStillRunningIsReportedAsNotClosed(t *testing.T) {
	server := oneMessageFeed(t, `{"k":{
		"t":1788404700000,"s":"BTCUSDT","o":"100","c":"115","h":"120","l":"90",
		"v":"1","q":"1","V":"1","Q":"1","x":false}}`)

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), "BTCUSDT")
	require.NoError(t, followError)

	assert.False(t, (<-liveKCandles).Closed)
}

// The feed ending is the only way this layer reports that it has ended, so a source
// saying something unreadable must end it rather than pass a half-read candle on.
func TestAnUnreadableMessageEndsTheFeed(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{name: "根本不是訊息", message: `not json at all`},
		{name: "價格不是數字", message: `{"k":{"t":1788404700000,"s":"BTCUSDT","o":"約一百",` +
			`"c":"115","h":"120","l":"90","v":"1","q":"1","V":"1","Q":"1","x":false}}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := oneMessageFeed(t, testCase.message)

			liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
				FollowKCandles(t.Context(), "BTCUSDT")
			require.NoError(t, followError)

			_, isDelivering := <-liveKCandles
			assert.False(t, isDelivering, "讀不懂的訊息應該結束這條通道，而不是往下傳半根 K 線")
		})
	}
}

// A source that will not have us must say so at once, so the caller can decide when
// to try again rather than waiting on a channel that will never speak.
func TestASourceThatCannotBeReachedIsReportedImmediately(t *testing.T) {
	testCases := []struct {
		name    string
		baseUrl string
		symbol  string
	}{
		{name: "沒有指定交易標的", baseUrl: "ws://127.0.0.1:1", symbol: "   "},
		{name: "位址無法解讀", baseUrl: "://not a url", symbol: "BTCUSDT"},
		{name: "連不上", baseUrl: "ws://127.0.0.1:1", symbol: "BTCUSDT"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			executionContext, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(testCase.baseUrl).
				FollowKCandles(executionContext, testCase.symbol)

			require.Error(t, followError)
			assert.Nil(t, liveKCandles)
		})
	}
}
