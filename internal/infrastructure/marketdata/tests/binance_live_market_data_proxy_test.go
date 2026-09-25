package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneMessageFeed sends one message and hangs up.
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

// Quietly following only the first of several symbols would leave the rest looking followed but never moving.
func TestThisSourceRefusesAChannelCarryingMoreThanOneSymbol(t *testing.T) {
	testCases := []struct {
		name    string
		symbols []string
	}{
		{name: "兩檔", symbols: []string{"BTCUSDT", "ETHUSDT"}},
		{name: "一檔都沒有", symbols: []string{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, followError := marketdata.NewBinanceLiveMarketDataProxy("ws://127.0.0.1:1").
				FollowKCandles(t.Context(),
					vo.NewLiveFollowChannelVo(vo.MarketCrypto, testCase.symbols))

			require.Error(t, followError)
			assert.Contains(t, followError.Error(), "one symbol to a channel")
		})
	}
}

func TestTheFeedIsOpenedForOneMinuteCandlesOfThatSymbol(t *testing.T) {
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
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
	require.NoError(t, followError)

	assert.Equal(t, "/btcusdt@kline_1m", <-askedFor)
}

// aLiveMessage includes every field the venue sends, including unread lookalikes ("t"/"T", "l"/"L"), so the test cannot share the code's blind spots.
func aLiveMessage(closed bool, low string) string {
	return fmt.Sprintf(`{"e":"kline","E":1788404712345,"s":"BTCUSDT","k":{
		"t":1788404700000,"T":1788404999999,"s":"BTCUSDT","i":"1m",
		"f":100,"L":200,"o":"100.5","c":"118.25","h":"120","l":%q,
		"v":"12.5","n":100,"x":%t,"q":"1400.75","V":"7.25","Q":"800.5","B":"0"}}`, low, closed)
}

func TestALiveMessageIsNormalizedIntoOneCandle(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(true, "90"))

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
	require.NoError(t, followError)

	liveKCandle := <-liveKCandles

	assert.Equal(t, "BTCUSDT", liveKCandle.Symbol)
	assert.Equal(t, time.UnixMilli(1788404700000).UTC(), liveKCandle.OpenTime)
	assert.Equal(t, "100.5", liveKCandle.Open.String())
	assert.Equal(t, "120", liveKCandle.High.String())
	assert.Equal(t, "90", liveKCandle.Low.String())
	assert.Equal(t, "118.25", liveKCandle.Close.String())
	assert.Equal(t, "12.5", liveKCandle.Volume.String())
	assert.Equal(t, "1400.75", liveKCandle.QuoteVolume.Decimal.String())
	assert.Equal(t, "7.25", liveKCandle.TakerBuyBaseVolume.Decimal.String())
	assert.Equal(t, "800.5", liveKCandle.TakerBuyQuoteVolume.Decimal.String())
	assert.True(t, liveKCandle.Closed, "來源說這一根走完了")
}

// Undeclared keys match their lookalike case-insensitively; the "T" into "t" case is silent and stamps candles one interval late.
func TestTheLookalikeFieldsDoNotLandInEachOther(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(false, "90"))

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
	require.NoError(t, followError)

	liveKCandle, isDelivering := <-liveKCandles

	require.True(t, isDelivering, "帶著最後成交編號的訊息不該讓整條通道結束")
	assert.Equal(t, time.UnixMilli(1788404700000).UTC(), liveKCandle.OpenTime,
		"起始時間必須是這一根開始的時刻，不是它結束的時刻")
	assert.Equal(t, "90", liveKCandle.Low.String(),
		"最低價必須是最低價，不是最後那一筆成交的編號")
}

// Downstream storage decisions depend on this flag.
func TestACandleStillRunningIsReportedAsNotClosed(t *testing.T) {
	server := oneMessageFeed(t, aLiveMessage(false, "90"))

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
	require.NoError(t, followError)

	assert.False(t, (<-liveKCandles).Closed)
}

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
				FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
			require.NoError(t, followError)

			_, isDelivering := <-liveKCandles
			assert.False(t, isDelivering, "讀不懂的訊息應該結束這條通道，而不是往下傳半根 K 線")
		})
	}
}

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
				FollowKCandles(executionContext, vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{testCase.symbol}))

			require.Error(t, followError)
			assert.Nil(t, liveKCandles)
		})
	}
}

// The contract venue's live candle uses the same message, so the contract follow reuses this proxy; pinned against the contract venue's spelling.
func TestAContractVenueLiveCandleIsReadTheSameWay(t *testing.T) {
	server := oneMessageFeed(t, `{"e":"kline","E":1788404760000,"s":"BTCUSDT","k":{`+
		`"t":1788404700000,"T":1788404759999,"s":"BTCUSDT","i":"1m","f":100,"L":200,`+
		`"o":"64000.1","c":"64000.5","h":"64010","l":"63990","v":"12.5","n":101,"x":false,`+
		`"q":"800006.25","V":"7.25","Q":"464000.9","B":"0"}}`)

	liveKCandles, followError := marketdata.NewBinanceLiveMarketDataProxy(streamUrlOf(server)).
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"BTCUSDT"}))
	require.NoError(t, followError)

	liveKCandle := <-liveKCandles

	assert.Equal(t, "BTCUSDT", liveKCandle.Symbol)
	assert.Equal(t, time.UnixMilli(1788404700000).UTC(), liveKCandle.OpenTime)
	assert.Equal(t, "64000.1", liveKCandle.Open.String())
	assert.Equal(t, "64010", liveKCandle.High.String())
	assert.Equal(t, "63990", liveKCandle.Low.String())
	assert.Equal(t, "64000.5", liveKCandle.Close.String())
	assert.Equal(t, "12.5", liveKCandle.Volume.String())
	assert.Equal(t, "800006.25", liveKCandle.QuoteVolume.Decimal.String())
	assert.Equal(t, "7.25", liveKCandle.TakerBuyBaseVolume.Decimal.String())
	assert.Equal(t, "464000.9", liveKCandle.TakerBuyQuoteVolume.Decimal.String())
	assert.False(t, liveKCandle.Closed)
}
