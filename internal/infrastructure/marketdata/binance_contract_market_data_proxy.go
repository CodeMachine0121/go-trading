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

// contractSourceRow carries the open time parsed once, since answers are aligned and paged by it.
type contractSourceRow struct {
	openTime time.Time
	kLine    binanceKLine
}

// The index price is queried by pair while every other series uses symbol; for perpetuals they are spelled the same.
const (
	symbolParameter = "symbol"
	pairParameter   = "pair"
)

// BinanceContractMarketDataProxy assembles each contract K candle from four endpoints aligned on open time; the traded klines decide which minutes exist, and other readings without one are dropped.
type BinanceContractMarketDataProxy struct {
	baseUrl         string
	markPriceUrl    string
	indexPriceUrl   string
	premiumIndexUrl string
	httpClient      *http.Client
	pacer           RequestPacer
}

func NewBinanceContractMarketDataProxy(
	baseUrl string,
	markPriceUrl string,
	indexPriceUrl string,
	premiumIndexUrl string,
	requestTimeout time.Duration,
	pacer RequestPacer,
) *BinanceContractMarketDataProxy {
	return &BinanceContractMarketDataProxy{
		baseUrl:         baseUrl,
		markPriceUrl:    markPriceUrl,
		indexPriceUrl:   indexPriceUrl,
		premiumIndexUrl: premiumIndexUrl,
		httpClient:      &http.Client{Timeout: requestTimeout},
		pacer:           pacer,
	}
}

// FetchKCandles returns candles oldest first with mark, index and premium readings where available; an empty window is not an error, but any failed request fails the whole call.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.ContractMarketKCandleVo, error) {
	tradedRows, tradedError := binanceContractMarketDataProxy.fetchRows(
		executionContext, binanceContractMarketDataProxy.baseUrl, symbolParameter, window)
	if tradedError != nil {
		return nil, tradedError
	}

	contractKCandles := make([]vo.ContractMarketKCandleVo, 0, len(tradedRows))
	for _, tradedRow := range tradedRows {
		contractKCandle, convertError := tradedRow.kLine.toContractMarketKCandleVo(window.Symbol)
		if convertError != nil {
			return nil, convertError
		}
		contractKCandles = append(contractKCandles, contractKCandle)
	}

	if len(contractKCandles) == 0 {
		// Nothing traded, so skip the other three requests to save the rate allowance.
		return contractKCandles, nil
	}

	markPrices, markError := binanceContractMarketDataProxy.fetchPriceLine(
		executionContext, binanceContractMarketDataProxy.markPriceUrl, symbolParameter, window)
	if markError != nil {
		return nil, markError
	}

	indexPrices, indexError := binanceContractMarketDataProxy.fetchPriceLine(
		executionContext, binanceContractMarketDataProxy.indexPriceUrl, pairParameter, window)
	if indexError != nil {
		return nil, indexError
	}

	premiumIndexes, premiumIndexError := binanceContractMarketDataProxy.fetchPriceLine(
		executionContext, binanceContractMarketDataProxy.premiumIndexUrl, symbolParameter, window)
	if premiumIndexError != nil {
		return nil, premiumIndexError
	}

	// A missing line is left absent rather than dropping the candle, so the domain can reject it and record which line was missing.
	for index, contractKCandle := range contractKCandles {
		openTime := contractKCandle.OpenTime.UnixMilli()

		if markPrice, hasMarkPrice := markPrices[openTime]; hasMarkPrice {
			contractKCandles[index].MarkOpen, contractKCandles[index].MarkHigh,
				contractKCandles[index].MarkLow, contractKCandles[index].MarkClose =
				markPrice.toNullDecimals()
		}

		if indexPrice, hasIndexPrice := indexPrices[openTime]; hasIndexPrice {
			contractKCandles[index].IndexOpen, contractKCandles[index].IndexHigh,
				contractKCandles[index].IndexLow, contractKCandles[index].IndexClose =
				indexPrice.toNullDecimals()
		}

		if premiumIndex, hasPremiumIndex := premiumIndexes[openTime]; hasPremiumIndex {
			contractKCandles[index].PremiumIndexOpen, contractKCandles[index].PremiumIndexHigh,
				contractKCandles[index].PremiumIndexLow, contractKCandles[index].PremiumIndexClose =
				premiumIndex.toNullDecimals()
		}
	}

	return contractKCandles, nil
}

// fetchPriceLine fetches a price-only series and keys it by open time.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) fetchPriceLine(
	executionContext context.Context,
	address string,
	contractParameter string,
	window vo.KCandleFetchWindowVo,
) (map[int64]priceLineFigures, error) {
	rows, fetchError := binanceContractMarketDataProxy.fetchRows(
		executionContext, address, contractParameter, window)
	if fetchError != nil {
		return nil, fetchError
	}

	pricesByOpenTime := make(map[int64]priceLineFigures, len(rows))
	for _, row := range rows {
		prices, convertError := row.kLine.toPriceLineFigures()
		if convertError != nil {
			return nil, convertError
		}
		pricesByOpenTime[row.openTime.UnixMilli()] = prices
	}

	return pricesByOpenTime, nil
}

// fetchRows pages until the source stops returning rows inside the window, so out-of-window rows cannot loop forever.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) fetchRows(
	executionContext context.Context,
	address string,
	contractParameter string,
	window vo.KCandleFetchWindowVo,
) ([]contractSourceRow, error) {
	rows := make([]contractSourceRow, 0)

	for nextStartTime := window.StartTime; !nextStartTime.After(window.EndTime); {
		page, fetchError := binanceContractMarketDataProxy.fetchPage(
			executionContext, address, contractParameter, window.Symbol, nextStartTime, window.EndTime)
		if fetchError != nil {
			return nil, fetchError
		}

		if len(page) == 0 {
			break
		}

		rows = append(rows, page...)
		nextStartTime = page[len(page)-1].openTime.Add(intervalStep)
	}

	return rows, nil
}

// fetchPage keeps only in-window rows and is separate so each response body is closed before the next page.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) fetchPage(
	executionContext context.Context,
	address string,
	contractParameter string,
	symbol string,
	startTime time.Time,
	endTime time.Time,
) ([]contractSourceRow, error) {
	if waitError := binanceContractMarketDataProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set(contractParameter, symbol)
	queryValues.Set("interval", kCandleInterval)
	queryValues.Set("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	queryValues.Set("endTime", strconv.FormatInt(endTime.UnixMilli(), 10))
	queryValues.Set("limit", strconv.Itoa(pageLimit))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		address+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach contract market source for %s: %w", symbol, buildError)
	}

	response, requestError := binanceContractMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach contract market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"contract market source answered %d for %s", response.StatusCode, symbol)
	}

	var kLines []binanceKLine

	answer := json.NewDecoder(response.Body)
	if decodeError := answer.Decode(&kLines); decodeError != nil {
		return nil, fmt.Errorf("read contract market source answer for %s: %w", symbol, decodeError)
	}

	// Trailing data after the array means an unreadable response, not an empty one.
	if answer.More() {
		return nil, fmt.Errorf(
			"read contract market source answer for %s: trailing content after the answer", symbol)
	}

	rowsInsideWindow := make([]contractSourceRow, 0, len(kLines))
	for _, kLine := range kLines {
		openTime, openTimeError := kLine.openTime()
		if openTimeError != nil {
			return nil, openTimeError
		}

		if openTime.Before(startTime) || openTime.After(endTime) {
			continue
		}
		rowsInsideWindow = append(rowsInsideWindow, contractSourceRow{openTime: openTime, kLine: kLine})
	}

	return rowsInsideWindow, nil
}
