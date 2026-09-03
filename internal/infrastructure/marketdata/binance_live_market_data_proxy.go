package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/coder/websocket"
)

// liveKCandleBufferSize lets the reader stay ahead of a domain busy storing a
// candle that just closed, without letting an unread backlog grow without bound.
const liveKCandleBufferSize = 16

// BinanceLiveMarketDataProxy follows one market over a connection that stays open.
// It is the only place that knows such a connection exists: everything above it
// sees a channel of candles that ends when the feed does.
//
// It reports one attempt and never retries. How long to wait before trying again is
// a rule stated in the requirements, so it lives in the domain where a table test
// can reach it.
type BinanceLiveMarketDataProxy struct {
	baseUrl string
}

func NewBinanceLiveMarketDataProxy(baseUrl string) *BinanceLiveMarketDataProxy {
	return &BinanceLiveMarketDataProxy{baseUrl: baseUrl}
}

// FollowKCandles opens the feed for one trading symbol and reports its candles
// until the feed ends, the context is done, or the source sends something
// unreadable. Closing the returned channel is the only way it says so.
func (binanceLiveMarketDataProxy *BinanceLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, symbol string,
) (<-chan vo.LiveKCandleVo, error) {
	streamUrl, urlError := binanceLiveMarketDataProxy.streamUrl(symbol)
	if urlError != nil {
		return nil, urlError
	}

	connection, _, dialError := websocket.Dial(executionContext, streamUrl, nil)
	if dialError != nil {
		return nil, fmt.Errorf("follow k candles for %s: %w", symbol, dialError)
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go binanceLiveMarketDataProxy.read(executionContext, connection, liveKCandles)

	return liveKCandles, nil
}

// read carries messages from the connection to the channel until either end stops.
// Whatever the reason, the connection is closed and the channel with it, so the
// caller learns of every ending in exactly one way.
func (binanceLiveMarketDataProxy *BinanceLiveMarketDataProxy) read(
	executionContext context.Context,
	connection *websocket.Conn,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)
	defer func() {
		if closeError := connection.CloseNow(); closeError != nil {
			log.Printf("live market data: closing the feed failed: %v", closeError)
		}
	}()

	for {
		_, body, readError := connection.Read(executionContext)
		if readError != nil {
			return
		}

		var message binanceLiveKLineMessage
		if decodeError := json.Unmarshal(body, &message); decodeError != nil {
			log.Printf("live market data: unreadable message: %v", decodeError)

			return
		}

		liveKCandle, conversionError := message.KLine.toLiveKCandleVo()
		if conversionError != nil {
			log.Printf("live market data: %v", conversionError)

			return
		}

		select {
		case liveKCandles <- liveKCandle:
		case <-executionContext.Done():
			return
		}
	}
}

// streamUrl spells one symbol's live K candle feed the way this source wants it:
// lowercase symbol, the candle length, joined by an underscore.
func (binanceLiveMarketDataProxy *BinanceLiveMarketDataProxy) streamUrl(symbol string) (string, error) {
	trimmedSymbol := strings.TrimSpace(symbol)
	if trimmedSymbol == "" {
		return "", fmt.Errorf("follow k candles: 請指定交易標的")
	}

	baseUrl, parseError := url.Parse(binanceLiveMarketDataProxy.baseUrl)
	if parseError != nil {
		return "", fmt.Errorf("follow k candles: %w", parseError)
	}

	return baseUrl.JoinPath(strings.ToLower(trimmedSymbol) + "@kline_" + kCandleInterval).String(), nil
}
