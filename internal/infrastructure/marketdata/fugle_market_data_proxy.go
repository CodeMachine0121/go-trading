package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// fugleTimeframe is how this source spells the length one K candle covers: plainly
// the number of minutes. Deriving it rather than writing it out keeps this source
// from quietly asking for a length the system no longer works in.
var fugleTimeframe = strconv.Itoa(domains.KCandleIntervalMinutes)

// fugleApiKeyHeader is where this source expects to be told who is asking.
const fugleApiKeyHeader = "X-API-KEY"

// FugleMarketDataProxy fetches Taiwan stock K candles from Fugle.
//
// Everything the rest of the system must not know about this source stops here: two
// addresses rather than one, dates said in Taipei rather than universal time, and
// the fact that "today" and "any earlier day" are asked for in different places.
//
// That split is the source's, not ours. Its intraday address only ever answers about
// today, and its historical address takes whole dates — so a window is asked for one
// local day at a time and trimmed back to what was actually wanted.
//
// **Each of those days costs a sequential, keyed request**, which is why this is
// handed the market itself rather than just its zone. Narrowing a window to the
// session only moves its two outer edges; the days in the middle it cannot trade on
// stay inside it. That cost nothing while the only lookback-driven window was a day
// or two, but a history sync hands over as many as ninety — a quarter of them
// weekends that can only ever answer empty, on a plan that starts answering 429 when
// pushed.
//
// **Which days those are is asked of the market, never decided here.** This is
// infrastructure; a copy of that rule living next to the requests would be a second
// answer to a question the domain already answers, and the two would drift on
// exactly the market nobody tested.
type FugleMarketDataProxy struct {
	intradayBaseUrl   string
	historicalBaseUrl string
	apiKey            string
	marketDomain      domains.MarketDomain
	clockProxy        _interface.IClockProxy
	httpClient        *http.Client
	pacer             requestPacer
}

func NewFugleMarketDataProxy(
	intradayBaseUrl string,
	historicalBaseUrl string,
	apiKey string,
	marketDomain domains.MarketDomain,
	clockProxy _interface.IClockProxy,
	requestTimeout time.Duration,
	requestsPerMinute int,
) *FugleMarketDataProxy {
	return &FugleMarketDataProxy{
		intradayBaseUrl:   intradayBaseUrl,
		historicalBaseUrl: historicalBaseUrl,
		apiKey:            apiKey,
		marketDomain:      marketDomain,
		clockProxy:        clockProxy,
		httpClient:        &http.Client{Timeout: requestTimeout},
		pacer:             newRequestPacer(requestsPerMinute),
	}
}

// FetchKCandles returns every K candle this source holds inside the window, oldest
// first. A window the source has nothing for is an empty result, not a failure.
func (fugleMarketDataProxy *FugleMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketKCandles := make([]vo.MarketKCandleVo, 0)

	today := fugleMarketDataProxy.clockProxy.Now().In(fugleMarketDataProxy.marketDomain.Zone())
	lastLocalDay := fugleMarketDataProxy.localDayOf(window.EndTime)

	for localDay := fugleMarketDataProxy.localDayOf(window.StartTime); !localDay.After(lastLocalDay); localDay = localDay.AddDate(0, 0, 1) {
		// Asked of the market, one whole local day at a time. A day it holds no
		// trading on can only ever answer empty, and this source charges a request
		// for finding that out.
		if !fugleMarketDataProxy.marketDomain.HoldsTrading(localDay, localDay.AddDate(0, 0, 1)) {
			continue
		}

		dayKCandles, fetchError := fugleMarketDataProxy.fetchDay(
			executionContext, window.Symbol, localDay, today)
		if fetchError != nil {
			return nil, fetchError
		}

		for _, dayKCandle := range dayKCandles {
			if dayKCandle.OpenTime.Before(window.StartTime) || dayKCandle.OpenTime.After(window.EndTime) {
				continue
			}
			marketKCandles = append(marketKCandles, dayKCandle)
		}
	}

	return marketKCandles, nil
}

// fetchDay asks for one local day, from whichever of the two addresses answers about
// that day.
func (fugleMarketDataProxy *FugleMarketDataProxy) fetchDay(
	executionContext context.Context,
	symbol string,
	localDay time.Time,
	today time.Time,
) ([]vo.MarketKCandleVo, error) {
	if waitError := fugleMarketDataProxy.pacer.waitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("timeframe", fugleTimeframe)
	// Oldest first, said out loud: the historical address answers newest first unless
	// told otherwise, and a caller downstream that assumed order would be reading the
	// day backwards without anything looking wrong.
	queryValues.Set("sort", "asc")

	baseUrl := fugleMarketDataProxy.historicalBaseUrl
	if fugleMarketDataProxy.isSameLocalDay(localDay, today) {
		// Today is only answered about at the intraday address. Asking the historical
		// one for today can come back empty long after the market has traded, which
		// would read as a quiet day and, for a whole market, as a holiday.
		baseUrl = fugleMarketDataProxy.intradayBaseUrl
	} else {
		requestedDate := localDay.Format(time.DateOnly)
		queryValues.Set("from", requestedDate)
		queryValues.Set("to", requestedDate)
	}

	return fugleMarketDataProxy.ask(
		executionContext, baseUrl+"/"+url.PathEscape(symbol), queryValues, symbol)
}

// ask makes one request and normalizes whatever it answers with.
func (fugleMarketDataProxy *FugleMarketDataProxy) ask(
	executionContext context.Context,
	requestUrl string,
	queryValues url.Values,
	symbol string,
) ([]vo.MarketKCandleVo, error) {
	request, buildError := http.NewRequestWithContext(
		executionContext, http.MethodGet, requestUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fugleMarketDataProxy.apiKey)

	response, requestError := fugleMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("market source answered %d for %s", response.StatusCode, symbol)
	}

	var candlesAnswer fugleCandlesAnswer

	answer := json.NewDecoder(response.Body)
	if decodeError := answer.Decode(&candlesAnswer); decodeError != nil {
		return nil, fmt.Errorf("read market source answer for %s: %w", symbol, decodeError)
	}

	// A decoder stops at the end of the first value and would ignore whatever came
	// after it. Ignored, a good answer followed by junk reads as a market with
	// nothing to report rather than as a source that cannot be read.
	if answer.More() {
		return nil, fmt.Errorf(
			"read market source answer for %s: trailing content after the answer", symbol)
	}

	marketKCandles := make([]vo.MarketKCandleVo, 0, len(candlesAnswer.Data))
	for _, reportedCandle := range candlesAnswer.Data {
		marketKCandle, convertError := reportedCandle.toMarketKCandleVo(symbol)
		if convertError != nil {
			return nil, convertError
		}
		marketKCandles = append(marketKCandles, marketKCandle)
	}

	return marketKCandles, nil
}

// localDayOf is the start of the local day a moment falls on, which is the unit this
// source answers in.
func (fugleMarketDataProxy *FugleMarketDataProxy) localDayOf(moment time.Time) time.Time {
	localMoment := moment.In(fugleMarketDataProxy.marketDomain.Zone())

	return time.Date(
		localMoment.Year(), localMoment.Month(), localMoment.Day(),
		0, 0, 0, 0, fugleMarketDataProxy.marketDomain.Zone())
}

func (fugleMarketDataProxy *FugleMarketDataProxy) isSameLocalDay(
	first time.Time, second time.Time,
) bool {
	return fugleMarketDataProxy.localDayOf(first).Equal(fugleMarketDataProxy.localDayOf(second))
}
