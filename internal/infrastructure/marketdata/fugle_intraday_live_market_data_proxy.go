package marketdata

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// FugleIntradayLiveMarketDataProxy follows Taiwan stocks by asking the same venue the
// stored history comes from what today's candles look like, over and over.
//
// **It asks for candles rather than assembling them.** An earlier attempt built them
// from the exchange's public quote feed, which turned out not to carry what that
// needs: measured during a session, its snapshots ran ten to sixty seconds behind,
// went backwards between consecutive requests as different nodes answered, and left
// the last-trade price untouched through hundreds of lots of trading. Candles built
// on that are wrong in ways nothing downstream can detect. This venue publishes the
// minute already formed, correct, and complete.
//
// **Asking the history venue also settles the figures for good.** A live update and
// the stored candle it lands beside are now the same venue describing the same
// minute, in the same unit, with the same meaning — so a figure that differed between
// them could only be one this system invented. The two bugs that cost this feature a
// morning were both of that kind.
//
// The price of the change is honest and worth writing down: this venue answers about
// one symbol per request and holds requests to an allowance, so how many symbols can
// be followed is bounded by that allowance rather than unbounded. It is still several
// times what the subscription feed allowed.
//
// It reports one attempt and never retries. How long to wait before trying again is a
// rule the requirements state, so it lives in the domain where a table test can reach
// it.
type FugleIntradayLiveMarketDataProxy struct {
	candlesUrl string
	apiKey     string
	// pollInterval is how often a follow asks, before the allowance is taken into
	// account. What is actually used is worked out per follow — see effectiveInterval.
	pollInterval time.Duration
	// requestsPerMinute is this venue's published allowance, shared with everything
	// else this system asks of it.
	requestsPerMinute int
	// quietTimeout is the silence a caller reads as a dead feed. It is held here only
	// to refuse, out loud, a configuration whose rounds could not beat it.
	quietTimeout time.Duration
	httpClient   *http.Client
	pacer        RequestPacer
}

func NewFugleIntradayLiveMarketDataProxy(
	candlesUrl string,
	apiKey string,
	pollInterval time.Duration,
	requestsPerMinute int,
	quietTimeout time.Duration,
	requestTimeout time.Duration,
	pacer RequestPacer,
) *FugleIntradayLiveMarketDataProxy {
	return &FugleIntradayLiveMarketDataProxy{
		candlesUrl:        candlesUrl,
		apiKey:            apiKey,
		pollInterval:      pollInterval,
		requestsPerMinute: requestsPerMinute,
		quietTimeout:      quietTimeout,
		httpClient:        &http.Client{Timeout: requestTimeout},
		pacer:             pacer,
	}
}

// FollowKCandles opens a follow for one channel of trading symbols and reports their
// candles until the feed ends, the context is done, or the venue stops answering.
// Closing the returned channel is the only way it says so.
//
// The first round is fetched before anything is handed back, so a caller given a
// channel has been told the venue is answering.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	pollInterval := fugleIntradayLiveMarketDataProxy.effectiveInterval(len(channel.Symbols))

	firstRound, askError := fugleIntradayLiveMarketDataProxy.askAll(executionContext, channel)
	if askError != nil {
		return nil, askError
	}

	liveKCandles := make(chan vo.LiveKCandleVo, liveKCandleBufferSize)
	go fugleIntradayLiveMarketDataProxy.poll(
		executionContext, channel, pollInterval, firstRound, liveKCandles)

	return liveKCandles, nil
}

// effectiveInterval is how often this follow may actually ask, which is its own
// setting unless the venue's allowance will not stretch that far.
//
// One request buys one symbol here, so a channel of ten asking every three seconds
// would want two hundred requests a minute from an allowance of fifty-five. Left to
// the pacer, each round would simply take longer and longer with nothing said — and
// far enough out the rounds outlast the silence a caller reads as a dead feed, so
// watching more stocks would end with none of them updating. Worked out here, the
// slowdown is a number rather than a surprise.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) effectiveInterval(
	symbolCount int,
) time.Duration {
	if symbolCount <= 0 || fugleIntradayLiveMarketDataProxy.requestsPerMinute <= 0 {
		return fugleIntradayLiveMarketDataProxy.pollInterval
	}

	affordable := time.Duration(math.Ceil(
		float64(symbolCount) / float64(fugleIntradayLiveMarketDataProxy.requestsPerMinute) *
			float64(time.Minute)))

	pollInterval := max(fugleIntradayLiveMarketDataProxy.pollInterval, affordable)

	// Said out loud because from every other angle this looks like a market that has
	// gone quiet: the follow keeps reconnecting, and the reason is a watchlist the
	// venue's allowance cannot keep up with rather than anything wrong with the feed.
	if fugleIntradayLiveMarketDataProxy.quietTimeout > 0 &&
		pollInterval >= fugleIntradayLiveMarketDataProxy.quietTimeout {
		log.Printf(
			"live market data: %d symbols need a round every %s, which is longer than "+
				"the %s a caller waits before giving a feed up for dead — follow fewer "+
				"symbols or raise the venue allowance",
			symbolCount, pollInterval, fugleIntradayLiveMarketDataProxy.quietTimeout)
	}

	return pollInterval
}

// poll carries the first round and then every later one to the channel, until either
// end stops. Whatever the reason, the channel closes, so the caller learns of every
// ending in exactly one way.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) poll(
	executionContext context.Context,
	channel vo.LiveFollowChannelVo,
	pollInterval time.Duration,
	firstRound map[string][]vo.LiveKCandleVo,
	liveKCandles chan<- vo.LiveKCandleVo,
) {
	defer close(liveKCandles)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// What each symbol's latest minute was last time round. A minute that is no longer
	// the latest is finished, and this venue has already stated it whole — so unlike a
	// feed that has to be assembled, nothing here is ever reported from a minute only
	// partly seen.
	latestMinuteBySymbol := make(map[string]time.Time, len(channel.Symbols))
	round := firstRound

	for {
		if !fugleIntradayLiveMarketDataProxy.publish(
			executionContext, round, latestMinuteBySymbol, liveKCandles) {
			return
		}

		select {
		case <-executionContext.Done():
			return
		case <-ticker.C:
		}

		polledRound, askError := fugleIntradayLiveMarketDataProxy.askAll(
			executionContext, channel)
		if askError != nil {
			// A follow the system ended on purpose is not a feed that broke, and
			// saying so would put a line in the log for every orderly shutdown.
			if executionContext.Err() == nil {
				log.Printf("live market data: the feed for %s ended: %v",
					channel.Key, askError)
			}

			return
		}

		round = polledRound
	}
}

// publish reports what one round changed: any minute that has just finished, then the
// one still forming. The finished one goes first so that no chart ever goes backwards.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) publish(
	executionContext context.Context,
	round map[string][]vo.LiveKCandleVo,
	latestMinuteBySymbol map[string]time.Time,
	liveKCandles chan<- vo.LiveKCandleVo,
) bool {
	for symbol, reportedKCandles := range round {
		if len(reportedKCandles) == 0 {
			continue
		}

		formingKCandle := reportedKCandles[len(reportedKCandles)-1]
		previousLatest, hasFollowed := latestMinuteBySymbol[symbol]
		latestMinuteBySymbol[symbol] = formingKCandle.OpenTime

		// The minute that was forming last round is finished the moment a later one
		// appears, and this venue states it whole — so it is stored as it stands here,
		// not as whatever part of it happened to be seen.
		if hasFollowed && formingKCandle.OpenTime.After(previousLatest) {
			finishedKCandle := reportedKCandles[len(reportedKCandles)-2]
			finishedKCandle.Closed = true

			if !send(executionContext, liveKCandles, finishedKCandle) {
				return false
			}
		}

		if !send(executionContext, liveKCandles, formingKCandle) {
			return false
		}
	}

	return true
}

// send passes one candle on, or reports that the follow is over.
func send(
	executionContext context.Context,
	liveKCandles chan<- vo.LiveKCandleVo,
	liveKCandle vo.LiveKCandleVo,
) bool {
	select {
	case liveKCandles <- liveKCandle:
		return true
	case <-executionContext.Done():
		return false
	}
}

// askAll asks about every symbol on the channel, one request each, and hands back
// what each is doing today.
//
// A symbol the venue will not answer about ends the whole round. The alternative —
// carrying on with the rest — would leave one symbol silently never updating while
// the feed looked healthy, which is the failure this whole feature exists to stop
// happening quietly.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) askAll(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (map[string][]vo.LiveKCandleVo, error) {
	round := make(map[string][]vo.LiveKCandleVo, len(channel.Symbols))
	for _, symbol := range channel.Symbols {
		reportedKCandles, askError := fugleIntradayLiveMarketDataProxy.ask(
			executionContext, symbol)
		if askError != nil {
			return nil, askError
		}

		round[symbol] = reportedKCandles
	}

	return round, nil
}

// ask makes one request for one symbol and normalizes whatever it answers with.
func (fugleIntradayLiveMarketDataProxy *FugleIntradayLiveMarketDataProxy) ask(
	executionContext context.Context, symbol string,
) ([]vo.LiveKCandleVo, error) {
	if waitError := fugleIntradayLiveMarketDataProxy.pacer.WaitForTurn(
		executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("timeframe", fugleTimeframe)

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		fugleIntradayLiveMarketDataProxy.candlesUrl+"/"+url.PathEscape(symbol)+
			"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fugleIntradayLiveMarketDataProxy.apiKey)

	response, requestError := fugleIntradayLiveMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, symbol)
	}

	candlesAnswer, decodeError := decodeFugleCandles(response.Body, symbol)
	if decodeError != nil {
		return nil, decodeError
	}

	liveKCandles := make([]vo.LiveKCandleVo, 0, len(candlesAnswer.Data))
	for _, reportedCandle := range candlesAnswer.Data {
		liveKCandle, convertError := reportedCandle.toLiveKCandleVo(symbol)
		if convertError != nil {
			return nil, convertError
		}
		liveKCandles = append(liveKCandles, liveKCandle)
	}

	return liveKCandles, nil
}
