package marketdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// fugleTimeframe is derived from the domain interval.
var fugleTimeframe = strconv.Itoa(domains.KCandleIntervalMinutes)

const fugleApiKeyHeader = "X-API-KEY"

// FugleMarketDataProxy fetches Taiwan stock K candles one Taipei-local day at a time (intraday address for today, historical for earlier days); it asks the market domain which days trade so non-trading days cost no request.
type FugleMarketDataProxy struct {
	intradayBaseUrl   string
	historicalBaseUrl string
	apiKey            string
	marketDomain      domains.MarketDomain
	clockProxy        _interface.IClockProxy
	httpClient        *http.Client
	pacer             RequestPacer
}

func NewFugleMarketDataProxy(
	intradayBaseUrl string,
	historicalBaseUrl string,
	apiKey string,
	marketDomain domains.MarketDomain,
	clockProxy _interface.IClockProxy,
	requestTimeout time.Duration,
	pacer RequestPacer,
) *FugleMarketDataProxy {
	return &FugleMarketDataProxy{
		intradayBaseUrl:   intradayBaseUrl,
		historicalBaseUrl: historicalBaseUrl,
		apiKey:            apiKey,
		marketDomain:      marketDomain,
		clockProxy:        clockProxy,
		httpClient:        &http.Client{Timeout: requestTimeout},
		pacer:             pacer,
	}
}

// FetchKCandles returns candles oldest first; an empty window is not an error.
func (fugleMarketDataProxy *FugleMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketKCandles := make([]vo.MarketKCandleVo, 0)

	today := fugleMarketDataProxy.clockProxy.Now().In(fugleMarketDataProxy.marketDomain.Zone())
	lastLocalDay := fugleMarketDataProxy.localDayOf(window.EndTime)

	for localDay := fugleMarketDataProxy.localDayOf(window.StartTime); !localDay.After(lastLocalDay); localDay = localDay.AddDate(0, 0, 1) {
		// Skip days with no trading, since each empty answer still costs a request.
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

func (fugleMarketDataProxy *FugleMarketDataProxy) fetchDay(
	executionContext context.Context,
	symbol string,
	localDay time.Time,
	today time.Time,
) ([]vo.MarketKCandleVo, error) {
	if waitError := fugleMarketDataProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("timeframe", fugleTimeframe)
	// The historical address defaults to newest first.
	queryValues.Set("sort", "asc")

	baseUrl := fugleMarketDataProxy.historicalBaseUrl
	if fugleMarketDataProxy.isSameLocalDay(localDay, today) {
		// Today must use the intraday address; the historical one can return empty for today long after trading.
		baseUrl = fugleMarketDataProxy.intradayBaseUrl
	} else {
		requestedDate := localDay.Format(time.DateOnly)
		queryValues.Set("from", requestedDate)
		queryValues.Set("to", requestedDate)
	}

	return fugleMarketDataProxy.ask(
		executionContext, baseUrl+"/"+url.PathEscape(symbol), queryValues, symbol)
}

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

	candlesAnswer, decodeError := decodeFugleCandles(response.Body, symbol)
	if decodeError != nil {
		return nil, decodeError
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
