package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/coder/websocket"
)

// fugleCandleInterval is the length one K candle covers, as a span. The source is
// told which symbol to push but not which length to push it at, so what arrives is
// folded into candles of this length here.
const fugleCandleInterval = 5 * time.Minute

// FugleLiveMarketDataProxy follows one Taiwan stock over a connection that stays
// open. It is the only place that knows such a connection exists: everything above it
// sees a channel of candles that ends when the feed does.
//
// It reports one attempt and never retries. How long to wait before trying again is a
// rule stated in the requirements, so it lives in the domain where a table test can
// reach it.
type FugleLiveMarketDataProxy struct {
	streamUrl string
	apiKey    string
}

func NewFugleLiveMarketDataProxy(streamUrl string, apiKey string) *FugleLiveMarketDataProxy {
	return &FugleLiveMarketDataProxy{streamUrl: streamUrl, apiKey: apiKey}
}

// FollowKCandles opens the feed for one trading symbol and reports its candles until
// the feed ends, the context is done, or the source sends something unreadable.
// Closing the returned channel is the only way it says so.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, target vo.FollowTargetVo,
) (<-chan vo.LiveKCandleVo, error) {
	connection, _, dialError := websocket.Dial(
		executionContext, fugleLiveMarketDataProxy.streamUrl, nil)
	if dialError != nil {
		return nil, fmt.Errorf("follow k candles for %s: %w", target.Symbol, dialError)
	}

	// Saying who is asking and what is wanted has to succeed before there is a feed
	// at all, so both are done here rather than in the reader — a caller handed a
	// channel is entitled to believe a feed was opened.
	if handshakeError := fugleLiveMarketDataProxy.handshake(
		executionContext, connection, target.Symbol); handshakeError != nil {
		_ = connection.CloseNow()

		return nil, handshakeError
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go fugleLiveMarketDataProxy.read(executionContext, connection, target.Symbol, liveKCandles)

	return liveKCandles, nil
}

// handshake tells the source who is asking and which symbol is wanted.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) handshake(
	executionContext context.Context, connection *websocket.Conn, symbol string,
) error {
	authentication := map[string]any{
		"event": "auth",
		"data":  map[string]any{"apikey": fugleLiveMarketDataProxy.apiKey},
	}
	if writeError := fugleLiveMarketDataProxy.send(
		executionContext, connection, authentication); writeError != nil {
		return fmt.Errorf("follow k candles for %s: %w", symbol, writeError)
	}

	subscription := map[string]any{
		"event": "subscribe",
		"data":  map[string]any{"channel": "candles", "symbol": symbol},
	}
	if writeError := fugleLiveMarketDataProxy.send(
		executionContext, connection, subscription); writeError != nil {
		return fmt.Errorf("follow k candles for %s: %w", symbol, writeError)
	}

	return nil
}

// send writes one instruction to the source. The loose map is the shape this source's
// protocol takes and it goes no further than this file.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) send(
	executionContext context.Context, connection *websocket.Conn, instruction map[string]any,
) error {
	body, encodeError := json.Marshal(instruction)
	if encodeError != nil {
		return encodeError
	}

	return connection.Write(executionContext, websocket.MessageText, body)
}

// read carries pushed candles to the channel until either end stops. Whatever the
// reason, the connection is closed and the channel with it, so the caller learns of
// every ending in exactly one way.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) read(
	executionContext context.Context,
	connection *websocket.Conn,
	symbol string,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)
	defer func() {
		if closeError := connection.CloseNow(); closeError != nil {
			log.Printf("live market data: closing the feed failed: %v", closeError)
		}
	}()

	forming := newFugleFormingKCandle(symbol)

	for {
		_, body, readError := connection.Read(executionContext)
		if readError != nil {
			return
		}

		var message fugleStreamMessage
		if decodeError := json.Unmarshal(body, &message); decodeError != nil {
			log.Printf("live market data: unreadable message: %v", decodeError)

			continue
		}

		// Everything that is not a pushed candle — the greeting, the acknowledgements,
		// the heartbeats — is the protocol talking about itself and says nothing about
		// the market.
		if message.Event != "data" || message.Channel != "candles" {
			continue
		}

		reportedKCandle, convertError := message.Data.toLiveKCandleVo(symbol)
		if convertError != nil {
			log.Printf("live market data: unreadable candle: %v", convertError)

			continue
		}

		for _, liveKCandle := range forming.absorb(reportedKCandle) {
			select {
			case liveKCandles <- liveKCandle:
			case <-executionContext.Done():
				return
			}
		}
	}
}
