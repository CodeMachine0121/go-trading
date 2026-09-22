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
	"github.com/shopspring/decimal"
)

// contractSourceRow is one row of a source answer together with the open time already
// read out of it. The open time is what both answers are aligned on and what paging
// steps forward from, so it is read once, at the boundary, and carried rather than
// parsed again by everyone who needs it.
type contractSourceRow struct {
	openTime time.Time
	kLine    binanceKLine
}

// BinanceContractMarketDataProxy fetches perpetual contract K candles from Binance.
//
// Everything the rest of the system must not know about this source stops here: the
// two addresses, the way it spells an interval, its positional wire format, the fact
// that a wide window has to be asked for in several goes — and the fact that one
// contract candle takes two questions rather than one.
//
// The two answers are aligned on open time. The traded half decides which minutes
// exist at all, because it is the one that says whether the market was there; a mark
// price for a minute with no trading half is a reading of nothing and is dropped.
type BinanceContractMarketDataProxy struct {
	baseUrl      string
	markPriceUrl string
	httpClient   *http.Client
	pacer        RequestPacer
}

func NewBinanceContractMarketDataProxy(
	baseUrl string, markPriceUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractMarketDataProxy {
	return &BinanceContractMarketDataProxy{
		baseUrl:      baseUrl,
		markPriceUrl: markPriceUrl,
		httpClient:   &http.Client{Timeout: requestTimeout},
		pacer:        pacer,
	}
}

// FetchKCandles returns every contract K candle the source holds inside the window,
// oldest first, each carrying its mark price where the source had one.
//
// A window the source has nothing for is an empty result, not a failure — a contract
// that did not yet exist over the stretch asked about produces exactly that. Either
// question failing fails the whole call, because half a contract candle is not a
// partial answer, it is a candle nobody can tell from a spot one.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.ContractMarketKCandleVo, error) {
	tradedRows, tradedError := binanceContractMarketDataProxy.fetchRows(
		executionContext, binanceContractMarketDataProxy.baseUrl, window)
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
		// Nothing traded over the stretch, so there is nothing a mark price could
		// belong to. Asking anyway would spend the venue's allowance on an answer
		// with no home, and long backfills are made of stretches like this one.
		return contractKCandles, nil
	}

	markRows, markError := binanceContractMarketDataProxy.fetchRows(
		executionContext, binanceContractMarketDataProxy.markPriceUrl, window)
	if markError != nil {
		return nil, markError
	}

	// Keyed by open time, which is the only thing the two answers have in common.
	markPricesByOpenTime := make(map[int64]markPriceFigures, len(markRows))
	for _, markRow := range markRows {
		markPrice, convertError := markRow.kLine.toMarkPriceFigures()
		if convertError != nil {
			return nil, convertError
		}
		markPricesByOpenTime[markRow.openTime.UnixMilli()] = markPrice
	}

	for index, contractKCandle := range contractKCandles {
		markPrice, hasMarkPrice := markPricesByOpenTime[contractKCandle.OpenTime.UnixMilli()]
		if !hasMarkPrice {
			continue
		}

		contractKCandles[index].MarkOpen = decimal.NewNullDecimal(markPrice.open)
		contractKCandles[index].MarkHigh = decimal.NewNullDecimal(markPrice.high)
		contractKCandles[index].MarkLow = decimal.NewNullDecimal(markPrice.low)
		contractKCandles[index].MarkClose = decimal.NewNullDecimal(markPrice.close)
	}

	return contractKCandles, nil
}

// fetchRows walks one address across the whole window, page by page, and hands back
// the raw rows in the order the source gave them.
//
// It keeps asking until the source stops producing rows inside the window, so a
// window wider than one page still comes back whole while a source that answers with
// rows outside it cannot keep the asking going.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) fetchRows(
	executionContext context.Context, address string, window vo.KCandleFetchWindowVo,
) ([]contractSourceRow, error) {
	rows := make([]contractSourceRow, 0)

	for nextStartTime := window.StartTime; !nextStartTime.After(window.EndTime); {
		page, fetchError := binanceContractMarketDataProxy.fetchPage(
			executionContext, address, window.Symbol, nextStartTime, window.EndTime)
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

// fetchPage asks one address once, keeping only the rows that actually fall inside
// the stretch it was asked for.
//
// **It has one caller and stays a method of its own because of what it encloses: one
// answer's body, from the moment it arrives to the moment it is let go.** Folded back
// into the loop above, the deferred close would not run until every page had been
// fetched — so a stretch of years would hold a thousand answer bodies open at once,
// and each of the four ways out of here would have to remember to close by hand.
func (binanceContractMarketDataProxy *BinanceContractMarketDataProxy) fetchPage(
	executionContext context.Context,
	address string,
	symbol string,
	startTime time.Time,
	endTime time.Time,
) ([]contractSourceRow, error) {
	if waitError := binanceContractMarketDataProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("symbol", symbol)
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

	// A decoder stops at the end of the first value and would ignore whatever came
	// after it. Ignored, a valid but empty array followed by junk reads as "the source
	// has nothing for this window" rather than as a source that cannot be read.
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
