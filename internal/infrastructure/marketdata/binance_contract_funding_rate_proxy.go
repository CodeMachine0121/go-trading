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

const fundingRatePageLimit = 1000

// earliestFundingRateSettlement stands in for "from the start" because the venue treats a zero start time as absent and returns the latest page.
var earliestFundingRateSettlement = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

// binanceFundingRateSettlement's mark price is empty on the oldest settlements.
type binanceFundingRateSettlement struct {
	Symbol      string `json:"symbol"`
	FundingTime int64  `json:"fundingTime"`
	FundingRate string `json:"fundingRate"`
	MarkPrice   string `json:"markPrice"`
}

// BinanceContractFundingRateProxy fetches funding rate settlements; it shares the contract K candles' pacer because the venue counts both against one budget.
type BinanceContractFundingRateProxy struct {
	fundingRateUrl string
	httpClient     *http.Client
	pacer          RequestPacer
}

func NewBinanceContractFundingRateProxy(
	fundingRateUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractFundingRateProxy {
	return &BinanceContractFundingRateProxy{
		fundingRateUrl: fundingRateUrl,
		httpClient:     &http.Client{Timeout: requestTimeout},
		pacer:          pacer,
	}
}

// FetchFundingRateSettlements returns every settlement strictly after the given moment, oldest first, across as many pages as needed.
func (binanceContractFundingRateProxy *BinanceContractFundingRateProxy) FetchFundingRateSettlements(
	executionContext context.Context, symbol string, after time.Time,
) ([]vo.ContractFundingRateSettlementVo, error) {
	nextStartTime := earliestFundingRateSettlement
	if !after.IsZero() {
		// The venue's start is inclusive and the caller already has the settlement at `after`.
		nextStartTime = after.Add(time.Millisecond)
	}

	settlements := make([]vo.ContractFundingRateSettlementVo, 0)
	for {
		page, fetchError := binanceContractFundingRateProxy.fetchPage(
			executionContext, symbol, nextStartTime)
		if fetchError != nil {
			return nil, fetchError
		}

		for _, reportedSettlement := range page {
			settlement, convertError := reportedSettlement.toContractFundingRateSettlementVo(symbol)
			if convertError != nil {
				return nil, convertError
			}
			settlements = append(settlements, settlement)
		}

		// A short page is the last; a page that does not advance past the start also stops the loop.
		if len(page) < fundingRatePageLimit {
			return settlements, nil
		}

		lastSettlementTime := time.UnixMilli(page[len(page)-1].FundingTime).UTC()
		if !lastSettlementTime.After(nextStartTime) {
			return settlements, nil
		}
		nextStartTime = lastSettlementTime.Add(time.Millisecond)
	}
}

// fetchPage is separate so each page's response body is closed before the next is fetched.
func (binanceContractFundingRateProxy *BinanceContractFundingRateProxy) fetchPage(
	executionContext context.Context, symbol string, startTime time.Time,
) ([]binanceFundingRateSettlement, error) {
	if waitError := binanceContractFundingRateProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("symbol", symbol)
	queryValues.Set("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	queryValues.Set("limit", strconv.Itoa(fundingRatePageLimit))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		binanceContractFundingRateProxy.fundingRateUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach contract funding rate source for %s: %w", symbol, buildError)
	}

	response, requestError := binanceContractFundingRateProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach contract funding rate source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"contract funding rate source answered %d for %s", response.StatusCode, symbol)
	}

	var page []binanceFundingRateSettlement
	if decodeError := json.NewDecoder(response.Body).Decode(&page); decodeError != nil {
		return nil, fmt.Errorf("read contract funding rate answer for %s: %w", symbol, decodeError)
	}

	return page, nil
}

func (reportedSettlement binanceFundingRateSettlement) toContractFundingRateSettlementVo(
	symbol string,
) (vo.ContractFundingRateSettlementVo, error) {
	fundingRate, rateError := decimal.NewFromString(reportedSettlement.FundingRate)
	if rateError != nil {
		return vo.ContractFundingRateSettlementVo{}, fmt.Errorf(
			"read funding rate for %s: %w", symbol, rateError)
	}

	markPrice := decimal.NullDecimal{}
	if reportedSettlement.MarkPrice != "" {
		parsedMarkPrice, markPriceError := decimal.NewFromString(reportedSettlement.MarkPrice)
		if markPriceError != nil {
			return vo.ContractFundingRateSettlementVo{}, fmt.Errorf(
				"read settlement mark price for %s: %w", symbol, markPriceError)
		}
		markPrice = decimal.NewNullDecimal(parsedMarkPrice)
	}

	return vo.ContractFundingRateSettlementVo{
		Symbol:         symbol,
		SettlementTime: time.UnixMilli(reportedSettlement.FundingTime).UTC(),
		FundingRate:    fundingRate,
		MarkPrice:      markPrice,
	}, nil
}
