package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/coder/websocket"
)

// fugleCandleInterval is the length one K candle covers, as a span. The source is
// told which symbol to push but not which length to push it at, so what arrives is
// folded into candles of this length here — the one length the system works in.
const fugleCandleInterval = domains.KCandleInterval

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
	// followsEveningBoard asks for the after-hours board as well as the regular one.
	//
	// This source publishes each board separately and pushes only what was asked for,
	// so a venue that trades twice a day needs two subscriptions per symbol. Which
	// board is running at any moment is the domain's knowledge and is not repeated
	// here: both are subscribed, and the quiet one simply says nothing.
	followsEveningBoard bool
	// authenticationTimeout bounds the wait for the source to say it accepted us. A
	// source that answers nothing would otherwise hold the attempt open for ever,
	// and the caller's retry — the thing that recovers from a bad line — never runs.
	authenticationTimeout time.Duration
}

func NewFugleLiveMarketDataProxy(
	streamUrl string, apiKey string, authenticationTimeout time.Duration,
) *FugleLiveMarketDataProxy {
	return &FugleLiveMarketDataProxy{
		streamUrl:             streamUrl,
		apiKey:                apiKey,
		authenticationTimeout: authenticationTimeout,
	}
}

// NewFugleEveningBoardLiveMarketDataProxy follows a venue on this same source that
// trades an evening board as well as a day board — Taiwan index futures does — and so
// has to be asked for both.
func NewFugleEveningBoardLiveMarketDataProxy(
	streamUrl string, apiKey string, authenticationTimeout time.Duration,
) *FugleLiveMarketDataProxy {
	return &FugleLiveMarketDataProxy{
		streamUrl:             streamUrl,
		apiKey:                apiKey,
		authenticationTimeout: authenticationTimeout,
		followsEveningBoard:   true,
	}
}

// FollowKCandles opens the feed for one trading symbol and reports its candles until
// the feed ends, the context is done, or the source sends something unreadable.
// Closing the returned channel is the only way it says so.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	connection, _, dialError := websocket.Dial(
		executionContext, fugleLiveMarketDataProxy.streamUrl, nil)
	if dialError != nil {
		return nil, fmt.Errorf("follow k candles for %s: %w", channel.Key, dialError)
	}

	// Saying who is asking and what is wanted has to succeed before there is a feed
	// at all, so both are done here rather than in the reader — a caller handed a
	// channel is entitled to believe a feed was opened.
	if handshakeError := fugleLiveMarketDataProxy.handshake(
		executionContext, connection, channel.Symbols); handshakeError != nil {
		_ = connection.CloseNow()

		return nil, handshakeError
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go fugleLiveMarketDataProxy.read(executionContext, connection, channel, liveKCandles)

	return liveKCandles, nil
}

// handshake tells the source who is asking, once, waits until it says it accepted
// us, and only then asks for the symbols, one request each.
//
// Saying who is asking only once is the point of this whole feature: the plan limits
// how many lines may be open at a time, not how many symbols may travel on one.
//
// Waiting in between is not politeness. This source answers each instruction in the
// order it arrives, so a subscription sent in the same breath as the credentials is
// read before they have been accepted and is refused as a forbidden resource. The
// line then stays open and perfectly silent — which reads from the outside exactly
// like a market with nothing to report, and is why this failed for a whole session
// without saying why.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) handshake(
	executionContext context.Context, connection *websocket.Conn, symbols []string,
) error {
	authentication := map[string]any{
		"event": "auth",
		"data":  map[string]any{"apikey": fugleLiveMarketDataProxy.apiKey},
	}
	if writeError := fugleLiveMarketDataProxy.send(
		executionContext, connection, authentication); writeError != nil {
		return fmt.Errorf("follow k candles: %w", writeError)
	}

	authenticationContext, giveUpOnAuthentication := context.WithTimeout(
		executionContext, fugleLiveMarketDataProxy.authenticationTimeout)
	defer giveUpOnAuthentication()

	for {
		_, body, readError := connection.Read(authenticationContext)
		if readError != nil {
			return fmt.Errorf("authenticate with market source: %w", readError)
		}

		var acknowledgement fugleStreamAcknowledgement
		if decodeError := json.Unmarshal(body, &acknowledgement); decodeError != nil {
			return fmt.Errorf("read market source answer: %w", decodeError)
		}

		if acknowledgement.Event == fugleAuthenticatedEvent {
			break
		}
		if acknowledgement.Event == fugleErrorEvent {
			return fmt.Errorf(
				"authenticate with market source: %s", acknowledgement.Data.Message)
		}
	}

	for _, symbol := range symbols {
		for _, subscription := range fugleLiveMarketDataProxy.subscriptionsFor(symbol) {
			if writeError := fugleLiveMarketDataProxy.send(
				executionContext, connection, subscription); writeError != nil {
				return fmt.Errorf("follow k candles for %s: %w", symbol, writeError)
			}
		}
	}

	return nil
}

// subscriptionsFor is what has to be asked of this source to hear about one symbol:
// its regular board, and its evening board too where the venue has one.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) subscriptionsFor(
	symbol string,
) []map[string]any {
	subscriptions := []map[string]any{{
		"event": "subscribe",
		"data":  map[string]any{"channel": "candles", "symbol": symbol},
	}}

	if fugleLiveMarketDataProxy.followsEveningBoard {
		subscriptions = append(subscriptions, map[string]any{
			"event": "subscribe",
			"data": map[string]any{
				"channel": "candles", "symbol": symbol, "afterHours": true,
			},
		})
	}

	return subscriptions
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
//
// Closing is best effort and its failure is not reported: by the time this runs the
// connection has almost always already gone, which is precisely why the read
// stopped. What is worth saying is why it stopped, and that is said once, with the
// symbol it happened to.
func (fugleLiveMarketDataProxy *FugleLiveMarketDataProxy) read(
	executionContext context.Context,
	connection *websocket.Conn,
	channel vo.LiveFollowChannelVo,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)
	defer func() { _ = connection.CloseNow() }()

	// One folder per symbol. They cannot share one: folding is about which slot a
	// push falls in and what the slot before it closed at, and two symbols pushing
	// alternately would each keep proving the other's slot finished.
	formingBySymbol := make(map[string]*fugleFormingKCandle, len(channel.Symbols))
	for _, symbol := range channel.Symbols {
		formingBySymbol[symbol] = newFugleFormingKCandle(symbol)
	}

	for {
		_, body, readError := connection.Read(executionContext)
		if readError != nil {
			// A follow the system ended on purpose is not a feed that broke, and
			// saying so would put a line in the log for every orderly shutdown.
			if executionContext.Err() == nil {
				log.Printf("live market data: the feed for %s ended: %v", channel.Key, readError)
			}

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

		// The push names its own symbol, and that is the only thing that decides whose
		// candle it is. A push for something this channel never subscribed to is not
		// worth guessing about — it belongs to nobody here.
		forming, isFollowed := formingBySymbol[message.Data.Symbol]
		if !isFollowed {
			continue
		}

		reportedKCandle, convertError := message.Data.toLiveKCandleVo(message.Data.Symbol)
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
