package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleInterval is how one K candle's length is spelled for this source, and
// pageLimit is the most candles it will hand over at once.
const (
	kCandleInterval = "5m"
	pageLimit       = 1000
)

// intervalStep is the same length as a duration, used to step past the last candle
// a page ended on.
const intervalStep = 5 * time.Minute

// BinanceMarketDataProxy fetches K candles from Binance. Everything the rest of the
// system must not know about this source stops here: the address, the way it spells
// an interval, its positional wire format, and the fact that a wide window has to be
// asked for in several goes.
type BinanceMarketDataProxy struct {
	baseUrl    string
	httpClient *http.Client
}

func NewBinanceMarketDataProxy(baseUrl string, requestTimeout time.Duration) *BinanceMarketDataProxy {
	return &BinanceMarketDataProxy{
		baseUrl:    baseUrl,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// FetchKCandles returns every K candle the source holds inside the window, oldest
// first. It keeps asking until the source stops producing candles inside the window,
// so a window wider than one page still comes back whole while a source that answers
// with candles outside it cannot keep the asking going. A window the source has
// nothing for is an empty result, not a failure.
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

// fetchPage asks the source once and normalizes whatever it answers with, keeping
// only the candles that actually fall inside the stretch it was asked for.
func (binanceMarketDataProxy *BinanceMarketDataProxy) fetchPage(
	executionContext context.Context,
	symbol string,
	startTime time.Time,
	endTime time.Time,
) ([]vo.MarketKCandleVo, error) {
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

	// A decoder stops at the end of the first value and would ignore whatever came
	// after it — an error page a proxy appended to an otherwise good answer, say.
	// Ignored, a valid but empty array followed by junk reads as "the source has
	// nothing for this window" rather than as a source that cannot be read, and a
	// window with candles in it would be quietly recorded as having none.
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
