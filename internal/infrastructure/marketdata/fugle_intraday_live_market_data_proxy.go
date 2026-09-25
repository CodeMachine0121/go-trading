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

// FugleIntradayLiveMarketDataProxy follows Taiwan stocks by polling the history venue's intraday candles, which are complete and consistent with stored history (the public quote feed proved laggy and inconsistent); one request per symbol means followable symbols are bounded by the rate allowance, and it never retries.
type FugleIntradayLiveMarketDataProxy struct {
	candlesUrl string
	apiKey     string
	// pollInterval is the requested cadence; the actual one comes from effectiveInterval.
	pollInterval time.Duration
	// requestsPerMinute is the venue's allowance, shared with everything else calling it.
	requestsPerMinute int
	// quietTimeout is held only to refuse a configuration whose rounds could not beat it.
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

// FollowKCandles polls until the feed ends, the context is done, or the venue stops answering, then closes the channel; the first round is fetched before returning.
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

// effectiveInterval slows the poll to what the allowance can sustain for this many symbols, so the slowdown is explicit rather than silently stretched rounds.
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

	// Logged because a watchlist too large for the allowance otherwise looks like a quiet market that keeps reconnecting.
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

// poll closes the channel on any exit.
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

	// Each symbol's latest minute from the previous round; once a later minute appears, the earlier one is complete as stated by the venue.
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
			// Do not log an intentional shutdown.
			if executionContext.Err() == nil {
				log.Printf("live market data: the feed for %s ended: %v",
					channel.Key, askError)
			}

			return
		}

		round = polledRound
	}
}

// publish emits any just-finished minute before the forming one so charts never go backwards.
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

// askAll requests every symbol and fails the whole round if any fails, so a symbol cannot silently stop updating.
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
