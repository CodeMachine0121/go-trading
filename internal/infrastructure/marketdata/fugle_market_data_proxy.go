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
var fugleTimeframe = strconv.Itoa(int(domains.KCandleInterval / time.Minute))

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
// local day at a time and trimmed back to what was actually wanted. The days are few:
// a market's session is narrowed before this is reached, so even a backfill spans
// two or three.
type FugleMarketDataProxy struct {
	intradayBaseUrl   string
	historicalBaseUrl string
	apiKey            string
	location          *time.Location
	clockProxy        _interface.IClockProxy
	httpClient        *http.Client
}

func NewFugleMarketDataProxy(
	intradayBaseUrl string,
	historicalBaseUrl string,
	apiKey string,
	location *time.Location,
	clockProxy _interface.IClockProxy,
	requestTimeout time.Duration,
) *FugleMarketDataProxy {
	return &FugleMarketDataProxy{
		intradayBaseUrl:   intradayBaseUrl,
		historicalBaseUrl: historicalBaseUrl,
		apiKey:            apiKey,
		location:          location,
		clockProxy:        clockProxy,
		httpClient:        &http.Client{Timeout: requestTimeout},
	}
}

// FetchKCandles returns every K candle this source holds inside the window, oldest
// first. A window the source has nothing for is an empty result, not a failure.
func (fugleMarketDataProxy *FugleMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketKCandles := make([]vo.MarketKCandleVo, 0)

	today := fugleMarketDataProxy.clockProxy.Now().In(fugleMarketDataProxy.location)
	lastLocalDay := fugleMarketDataProxy.localDayOf(window.EndTime)

	for localDay := fugleMarketDataProxy.localDayOf(window.StartTime); !localDay.After(lastLocalDay); localDay = localDay.AddDate(0, 0, 1) {
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
	localMoment := moment.In(fugleMarketDataProxy.location)

	return time.Date(
		localMoment.Year(), localMoment.Month(), localMoment.Day(),
		0, 0, 0, 0, fugleMarketDataProxy.location)
}

func (fugleMarketDataProxy *FugleMarketDataProxy) isSameLocalDay(
	first time.Time, second time.Time,
) bool {
	return fugleMarketDataProxy.localDayOf(first).Equal(fugleMarketDataProxy.localDayOf(second))
}
