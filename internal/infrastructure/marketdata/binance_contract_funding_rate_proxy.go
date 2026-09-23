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

// fundingRatePageLimit is the most settlements the venue hands back in one answer.
const fundingRatePageLimit = 1000

// earliestFundingRateSettlement is where "from the very first settlement" is asked
// from. The venue does not take a start of zero — it reads it as no start at all and
// answers with the most recent page instead — so the question has to name a moment,
// and this one is earlier than any perpetual contract the venue has ever listed.
var earliestFundingRateSettlement = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

// binanceFundingRateSettlement is one settlement as the venue spells it. The mark
// price is a quoted decimal, or an empty string on the oldest settlements.
type binanceFundingRateSettlement struct {
	Symbol      string `json:"symbol"`
	FundingTime int64  `json:"fundingTime"`
	FundingRate string `json:"fundingRate"`
	MarkPrice   string `json:"markPrice"`
}

// BinanceContractFundingRateProxy fetches funding rate settlements from Binance's
// perpetual contract venue.
//
// It spends the same allowance as the contract K candles, because the venue counts
// both against one budget.
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

// FetchFundingRateSettlements returns every settlement strictly after the given
// moment, oldest first, walking as many pages as that takes.
func (binanceContractFundingRateProxy *BinanceContractFundingRateProxy) FetchFundingRateSettlements(
	executionContext context.Context, symbol string, after time.Time,
) ([]vo.ContractFundingRateSettlementVo, error) {
	nextStartTime := earliestFundingRateSettlement
	if !after.IsZero() {
		// The venue's start is inclusive, and the settlement at `after` is one this
		// caller already has.
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

		// A short page is the last one. Stepping past the last settlement read is
		// what moves the asking forward, so a page whose last settlement does not lie
		// past the start cannot keep it going either.
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

// fetchPage asks the venue once.
//
// **It has one caller and stays a method of its own because of what it encloses: one
// answer's body, from the moment it arrives to the moment it is let go.** Folded back
// into the loop above, the deferred close would not run until every page had been
// read, and the history of a contract listed years ago is several pages long.
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

// toContractFundingRateSettlementVo turns one reported settlement into the shape the
// domain accepts. The settlement time is kept to the millisecond the venue named.
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
