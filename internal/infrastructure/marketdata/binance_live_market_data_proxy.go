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

// liveKCandleBufferSize lets the reader run ahead of a busy consumer without an unbounded backlog.
const liveKCandleBufferSize = 16

// BinanceLiveMarketDataProxy follows one market over a websocket and exposes it as a channel that closes when the feed ends; it never retries, since backoff is a domain rule.
type BinanceLiveMarketDataProxy struct {
	baseUrl string
}

func NewBinanceLiveMarketDataProxy(baseUrl string) *BinanceLiveMarketDataProxy {
	return &BinanceLiveMarketDataProxy{baseUrl: baseUrl}
}

// FollowKCandles streams candles until the feed ends, the context is done, or a message is unreadable, then closes the channel; channels with more than one symbol are refused because combined streams are not supported.
func (binanceLiveMarketDataProxy *BinanceLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	if len(channel.Symbols) != 1 {
		return nil, fmt.Errorf(
			"follow k candles for %s: this source follows one symbol to a channel, asked for %d",
			channel.Market, len(channel.Symbols))
	}
	symbol := channel.Symbols[0]

	streamUrl, urlError := binanceLiveMarketDataProxy.streamUrl(symbol)
	if urlError != nil {
		return nil, urlError
	}

	connection, _, dialError := websocket.Dial(executionContext, streamUrl, nil)
	if dialError != nil {
		return nil, fmt.Errorf("follow k candles for %s: %w", symbol, dialError)
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go binanceLiveMarketDataProxy.read(executionContext, connection, symbol, liveKCandles)

	return liveKCandles, nil
}

// read closes both the connection and the channel on any exit, logging the reason once; close errors are ignored since the connection is usually already gone.
func (binanceLiveMarketDataProxy *BinanceLiveMarketDataProxy) read(
	executionContext context.Context,
	connection *websocket.Conn,
	symbol string,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)
	defer func() { _ = connection.CloseNow() }()

	for {
		_, body, readError := connection.Read(executionContext)
		if readError != nil {
			// Do not log an intentional shutdown.
			if executionContext.Err() == nil {
				log.Printf("live market data: the feed for %s ended: %v", symbol, readError)
			}

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

// streamUrl builds "<lowercase symbol>@kline_<interval>".
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
