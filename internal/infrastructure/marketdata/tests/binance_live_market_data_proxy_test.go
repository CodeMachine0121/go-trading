package marketdata_test

import (
	"context"
	"fmt"
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

// aLiveMessage is the message **as this source actually sends it** — every field it
// puts on the wire, not only the ones this layer reads.
//
// The fields nothing reads are the point. This source names two pairs of different
// things with the same letter in different cases: "t" opens the candle while "T"
// closes it, "l" is the low while "L" is the last trade's number. A message written
// from the same understanding as the code can only ever agree with it, which is
// exactly how those two got through.
func aLiveMessage(closed bool, low string) string {
	return fmt.Sprintf(`{"e":"kline","E":1788404712345,"s":"BTCUSDT","k":{
		"t":1788404700000,"T":1788404999999,"s":"BTCUSDT","i":"5m",
		"f":100,"L":200,"o":"100.5","c":"118.25","h":"120","l":%q,
		"v":"12.5","n":100,"x":%t,"q":"1400.75","V":"7.25","Q":"800.5","B":"0"}}`, low, closed)
}

// The live message names its fields in one or two letters. Translating them is this
// layer's whole job, and getting one wrong would put a price in a volume.
func TestALiveMessageIsNormalizedIntoOneCandle(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(true, "90"))

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

// This source names two pairs of different things with the same letter in different
// cases. Decoding matches a key case-insensitively once no field claims it exactly,
// so a field left undeclared does not get ignored — it lands in its lookalike.
//
// One of the two says so loudly and one says nothing at all, and the quiet one is
// the dangerous one: a candle stamped one interval late merges into the wrong
// candle, and the chart looks entirely normal while it does it.
func TestTheLookalikeFieldsDoNotLandInEachOther(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(false, "90"))

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), "BTCUSDT")
	require.NoError(t, followError)

	liveKCandle, isDelivering := <-liveKCandles

	require.True(t, isDelivering, "帶著最後成交編號的訊息不該讓整條通道結束")
	assert.Equal(t, time.UnixMilli(1788404700000).UTC(), liveKCandle.OpenTime,
		"起始時間必須是這一根開始的時刻，不是它結束的時刻")
	assert.Equal(t, "90", liveKCandle.Low.String(),
		"最低價必須是最低價，不是最後那一筆成交的編號")
}

// A candle still running is the ordinary case, and it must be reported as such —
// everything downstream decides what may be stored from this one flag.
func TestACandleStillRunningIsReportedAsNotClosed(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(false, "90"))

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
		{name: "價格不是數字", message: aLiveMessage(false, "約九十")},
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
