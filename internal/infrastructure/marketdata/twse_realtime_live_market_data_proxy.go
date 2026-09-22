package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// twseListedBoardPrefixes are the boards this venue lists its stocks on, and the
// prefix each one is asked for under.
//
// Every symbol is asked for under both, because which board a code is listed on
// cannot be worked out from the code and the system does not record it. The board
// that does not list it answers with an empty entry, which costs one slot in the
// request and nothing else — and that is a great deal cheaper than being wrong about
// where a stock is listed.
var twseListedBoardPrefixes = []string{"tse_", "otc_"}

// twseRealtimeUserAgent is the one header this source is given.
//
// **Nothing else may be added, and a referer least of all.** This source answers a
// plain request and refuses one that arrives dressed as a browser — measured, not
// guessed: three requests carrying a referer were refused outright, and the same
// three without it were answered. The temptation to "make it look more like a real
// browser" is exactly how this stops working, so it is written down here rather than
// left to be rediscovered.
const twseRealtimeUserAgent = "Mozilla/5.0"

// TwseRealtimeLiveMarketDataProxy follows a channel of Taiwan stocks by asking the
// exchange what they are doing, over and over, for as long as the follow lasts.
//
// It is the only place that knows the feed is asked for rather than pushed. Above it
// sits a channel of candles that ends when the feed does — the same shape a socket
// would have handed back, which is why swapping one for the other changed nothing
// further in.
//
// It reports one attempt and never retries. How long to wait before trying again is
// a rule the requirements state, so it lives in the domain where a table test can
// reach it.
type TwseRealtimeLiveMarketDataProxy struct {
	quoteUrl string
	// marketZone is the zone this source states its trade times in. A time of day
	// with no zone names a different moment in every reader.
	marketZone   *time.Location
	pollInterval time.Duration
	httpClient   *http.Client
	pacer        RequestPacer
}

func NewTwseRealtimeLiveMarketDataProxy(
	quoteUrl string,
	marketDomain domains.MarketDomain,
	pollInterval time.Duration,
	requestTimeout time.Duration,
	pacer RequestPacer,
) *TwseRealtimeLiveMarketDataProxy {
	return &TwseRealtimeLiveMarketDataProxy{
		quoteUrl:     quoteUrl,
		marketZone:   marketDomain.Zone(),
		pollInterval: pollInterval,
		httpClient:   &http.Client{Timeout: requestTimeout},
		pacer:        pacer,
	}
}

// FollowKCandles opens the feed for one channel of trading symbols and reports their
// candles until the feed ends, the context is done, or the source stops answering.
// Closing the returned channel is the only way it says so.
//
// The first answer is fetched before anything is handed back, so a caller given a
// channel has been told the source is answering. A source that refuses from the
// outset is a failure to open, not a feed that opened and went quiet — and the two
// lead the caller to do different things.
func (twseRealtimeLiveMarketDataProxy *TwseRealtimeLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	// One folder per symbol. They cannot share one: folding is about which minute a
	// quote falls in and what the running total stood at when that minute opened, and
	// two symbols answering alternately would each keep resetting the other's.
	formingBySymbol := make(map[string]*twseFormingKCandle, len(channel.Symbols))
	for _, symbol := range channel.Symbols {
		formingBySymbol[symbol] = newTwseFormingKCandle(symbol)
	}

	firstQuotes, askError := twseRealtimeLiveMarketDataProxy.ask(executionContext, channel)
	if askError != nil {
		return nil, askError
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go twseRealtimeLiveMarketDataProxy.poll(
		executionContext, channel, formingBySymbol, firstQuotes, liveKCandles)

	return liveKCandles, nil
}

// poll carries the first answer and then every later one to the channel, until either
// end stops. Whatever the reason, the channel closes, so the caller learns of every
// ending in exactly one way.
func (twseRealtimeLiveMarketDataProxy *TwseRealtimeLiveMarketDataProxy) poll(
	executionContext context.Context,
	channel vo.LiveFollowChannelVo,
	formingBySymbol map[string]*twseFormingKCandle,
	firstQuotes []twseRealtimeQuote,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)

	ticker := time.NewTicker(twseRealtimeLiveMarketDataProxy.pollInterval)
	defer ticker.Stop()

	// The answer already in hand starts the loop rather than being published ahead of
	// it. Published separately, the first round would be a second copy of the body
	// below — and the copy could never do anything the loop does not, because the very
	// first answer about a symbol can only establish where its running total stood.
	quotes := firstQuotes

	for {
		if !twseRealtimeLiveMarketDataProxy.publish(
			executionContext, formingBySymbol, quotes, liveKCandles) {
			return
		}

		select {
		case <-executionContext.Done():
			return
		case <-ticker.C:
		}

		polledQuotes, askError := twseRealtimeLiveMarketDataProxy.ask(executionContext, channel)
		if askError != nil {
			// A follow the system ended on purpose is not a feed that broke, and
			// saying so would put a line in the log for every orderly shutdown.
			if executionContext.Err() == nil {
				log.Printf("live market data: the feed for %s ended: %v", channel.Key, askError)
			}

			return
		}

		quotes = polledQuotes
	}
}

// publish folds one answer into the candles it changes and sends them on, reporting
// whether the follow should carry on.
//
// A quote about a symbol this channel never asked for belongs to nobody here. Whether
// a quote says anything at all is the quote's own to answer — see toLiveKCandleVo for
// the two everyday ways it says nothing.
func (twseRealtimeLiveMarketDataProxy *TwseRealtimeLiveMarketDataProxy) publish(
	executionContext context.Context,
	formingBySymbol map[string]*twseFormingKCandle,
	quotes []twseRealtimeQuote,
	liveKCandles chan<- vo.LiveKCandleVo,
) bool {
	for _, quote := range quotes {
		forming, isFollowed := formingBySymbol[quote.Symbol]
		if !isFollowed {
			continue
		}

		quotedKCandle, hasQuote, convertError := quote.toLiveKCandleVo(
			twseRealtimeLiveMarketDataProxy.marketZone)
		if convertError != nil {
			log.Printf("live market data: unreadable quote: %v", convertError)

			continue
		}
		if !hasQuote {
			continue
		}

		for _, liveKCandle := range forming.absorb(quotedKCandle) {
			select {
			case liveKCandles <- liveKCandle:
			case <-executionContext.Done():
				return false
			}
		}
	}

	return true
}

// ask makes one request for the whole channel and normalizes whatever it answers
// with.
func (twseRealtimeLiveMarketDataProxy *TwseRealtimeLiveMarketDataProxy) ask(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) ([]twseRealtimeQuote, error) {
	if waitError := twseRealtimeLiveMarketDataProxy.pacer.WaitForTurn(
		executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("ex_ch", twseRealtimeLiveMarketDataProxy.channelQuery(channel))
	queryValues.Set("json", "1")
	// Asking for the undelayed figures out loud. The default is this source's to
	// choose and has changed before; a follow that quietly became delayed would look
	// exactly like a market that had gone quiet.
	queryValues.Set("delay", "0")

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		twseRealtimeLiveMarketDataProxy.quoteUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", channel.Key, buildError)
	}
	request.Header.Set("User-Agent", twseRealtimeUserAgent)

	response, requestError := twseRealtimeLiveMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", channel.Key, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, channel.Key)
	}

	var realtimeAnswer twseRealtimeAnswer

	answer := json.NewDecoder(response.Body)
	if decodeError := answer.Decode(&realtimeAnswer); decodeError != nil {
		return nil, fmt.Errorf(
			"read market source answer for %s: %w", channel.Key, decodeError)
	}

	// A decoder stops at the end of the first value and would ignore whatever came
	// after it. Ignored, a good answer followed by junk reads as a market with nothing
	// to report rather than as a source that cannot be read.
	if answer.More() {
		return nil, fmt.Errorf(
			"read market source answer for %s: trailing content after the answer",
			channel.Key)
	}

	// This source states its own verdict inside a body it sends with a perfectly
	// ordinary status. Reading only the status would take a refusal for a quiet market.
	if realtimeAnswer.ReturnCode != twseRealtimeOkReturnCode {
		return nil, fmt.Errorf("market source refused %s: %s %s",
			channel.Key, realtimeAnswer.ReturnCode, realtimeAnswer.ReturnMessage)
	}

	return realtimeAnswer.Quotes, nil
}

// channelQuery spells the whole channel the way this source expects to be asked:
// every symbol under every board it might be listed on, in one request.
func (twseRealtimeLiveMarketDataProxy *TwseRealtimeLiveMarketDataProxy) channelQuery(
	channel vo.LiveFollowChannelVo,
) string {
	askedSymbols := make([]string, 0, len(channel.Symbols)*len(twseListedBoardPrefixes))
	for _, symbol := range channel.Symbols {
		for _, boardPrefix := range twseListedBoardPrefixes {
			askedSymbols = append(askedSymbols, boardPrefix+symbol+".tw")
		}
	}

	return strings.Join(askedSymbols, "|")
}
