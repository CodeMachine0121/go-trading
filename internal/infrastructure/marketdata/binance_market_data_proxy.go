package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleInterval is derived from the domain interval; the "<n>m" spelling only holds for whole minutes under an hour.
var kCandleInterval = strconv.Itoa(domains.KCandleIntervalMinutes) + "m"

const pageLimit = 1000

const intervalStep = domains.KCandleInterval

// BinanceMarketDataProxy fetches spot K candles, hiding the address, interval spelling, positional wire format and paging.
type BinanceMarketDataProxy struct {
	baseUrl    string
	httpClient *http.Client
	pacer      RequestPacer
}

func NewBinanceMarketDataProxy(
	baseUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceMarketDataProxy {
	return &BinanceMarketDataProxy{
		baseUrl:    baseUrl,
		httpClient: &http.Client{Timeout: requestTimeout},
		pacer:      pacer,
	}
}

// FetchKCandles pages until the source stops returning candles inside the window, so out-of-window data cannot loop forever; an empty window is not an error.
func (binanceMarketDataProxy *BinanceMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketKCandles := make([]vo.MarketKCandleVo, 0)

	for nextStartTime := window.StartTime; !nextStartTime.After(window.EndTime); {
		page, fetchError := binanceMarketDataProxy.fetchPage(
			executionContext, window.Symbol, nextStartTime, window.EndTime)
		if fetchError != nil {
			return nil, fetchError
		}

		if len(page) == 0 {
			break
		}

		marketKCandles = append(marketKCandles, page...)
		nextStartTime = page[len(page)-1].OpenTime.Add(intervalStep)
	}

	return marketKCandles, nil
}

// fetchPage keeps only candles inside the requested stretch.
func (binanceMarketDataProxy *BinanceMarketDataProxy) fetchPage(
	executionContext context.Context,
	symbol string,
	startTime time.Time,
	endTime time.Time,
) ([]vo.MarketKCandleVo, error) {
	if waitError := binanceMarketDataProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("symbol", symbol)
	queryValues.Set("interval", kCandleInterval)
	queryValues.Set("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	queryValues.Set("endTime", strconv.FormatInt(endTime.UnixMilli(), 10))
	queryValues.Set("limit", strconv.Itoa(pageLimit))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		binanceMarketDataProxy.baseUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, buildError)
	}

	response, requestError := binanceMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("market source answered %d for %s", response.StatusCode, symbol)
	}

	var kLines []binanceKLine

	answer := json.NewDecoder(response.Body)
	if decodeError := answer.Decode(&kLines); decodeError != nil {
		return nil, fmt.Errorf("read market source answer for %s: %w", symbol, decodeError)
	}

	// Trailing data after the array (e.g. an appended error page) means an unreadable response, not an empty window.
	if answer.More() {
		return nil, fmt.Errorf(
			"read market source answer for %s: trailing content after the answer", symbol)
	}

	marketKCandles := make([]vo.MarketKCandleVo, 0, len(kLines))
	for _, kLine := range kLines {
		marketKCandle, convertError := kLine.toMarketKCandleVo(symbol)
		if convertError != nil {
			return nil, convertError
		}

		if marketKCandle.OpenTime.Before(startTime) || marketKCandle.OpenTime.After(endTime) {
			continue
		}
		marketKCandles = append(marketKCandles, marketKCandle)
	}

	return marketKCandles, nil
}
