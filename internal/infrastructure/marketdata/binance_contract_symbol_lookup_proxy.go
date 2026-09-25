package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// tradingStatus is the only status that yields candles; SETTLING and PENDING_TRADING contracts return nothing.
const tradingStatus = "TRADING"

// perpetualContractTypes lists the non-delivering contract types; any unrecognised type, including dated futures, is refused rather than assumed perpetual.
var perpetualContractTypes = map[string]bool{
	"PERPETUAL":         true,
	"TRADIFI_PERPETUAL": true,
}

// maintenanceMarginPercentScale converts the venue's percentage maintenance margin to a proportion; the liquidation fee is already a proportion.
var maintenanceMarginPercentScale = decimal.NewFromInt(100)

type binanceContractFilter struct {
	FilterType string `json:"filterType"`
	TickSize   string `json:"tickSize"`
	StepSize   string `json:"stepSize"`
	MinQty     string `json:"minQty"`
	Notional   string `json:"notional"`
}

type binanceContractListing struct {
	Symbol             string                  `json:"symbol"`
	Status             string                  `json:"status"`
	ContractType       string                  `json:"contractType"`
	MaintMarginPercent string                  `json:"maintMarginPercent"`
	LiquidationFee     string                  `json:"liquidationFee"`
	Filters            []binanceContractFilter `json:"filters"`
}

type binanceContractExchangeInfo struct {
	Symbols []binanceContractListing `json:"symbols"`
}

// binanceFundingInterval lists only contracts with a non-default funding interval.
type binanceFundingInterval struct {
	Symbol               string `json:"symbol"`
	FundingIntervalHours int    `json:"fundingIntervalHours"`
}

// BinanceContractSymbolLookupProxy scans the whole contract catalogue, because the venue ignores the symbol parameter and returns everything with a success; specifications also need the separate funding interval list.
type BinanceContractSymbolLookupProxy struct {
	baseUrl        string
	fundingInfoUrl string
	httpClient     *http.Client
	// The pacer is shared with every proxy on the contract venue, since the allowance is per venue.
	pacer RequestPacer
}

func NewBinanceContractSymbolLookupProxy(
	baseUrl string, fundingInfoUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractSymbolLookupProxy {
	return &BinanceContractSymbolLookupProxy{
		baseUrl:        baseUrl,
		fundingInfoUrl: fundingInfoUrl,
		httpClient:     &http.Client{Timeout: requestTimeout},
		pacer:          pacer,
	}
}

// LookUpSymbol accepts only trading perpetuals, since delisted contracts and dated futures stay in the catalogue but stop producing candles; it never sets a display name.
func (binanceContractSymbolLookupProxy *BinanceContractSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, symbol string,
) (vo.ContractSymbolListingVo, error) {
	exchangeInfo, catalogueError := binanceContractSymbolLookupProxy.fetchCatalogue(executionContext, symbol)
	if catalogueError != nil {
		return vo.ContractSymbolListingVo{}, catalogueError
	}

	for _, listing := range exchangeInfo.Symbols {
		if listing.Symbol != symbol {
			continue
		}

		if !listing.isFollowable() {
			return vo.ContractSymbolListingVo{}, nil
		}

		// If the funding interval list fails the contract is still followable, but no specification is returned rather than guessing the interval.
		fundingIntervals, intervalError := binanceContractSymbolLookupProxy.fetchFundingIntervals(
			executionContext, symbol)
		if intervalError != nil {
			return vo.ContractSymbolListingVo{IsListed: true}, nil
		}

		// An unreadable specification is returned empty and the daily refresh retries.
		specification, specificationError := listing.toContractTradingSpecificationVo(fundingIntervals)
		if specificationError != nil {
			return vo.ContractSymbolListingVo{IsListed: true}, nil
		}

		return vo.ContractSymbolListingVo{IsListed: true, Specification: specification}, nil
	}

	return vo.ContractSymbolListingVo{}, nil
}

// FetchTradingSpecifications returns specifications for trading perpetuals; unreadable or no-longer-listed contracts are absent.
func (binanceContractSymbolLookupProxy *BinanceContractSymbolLookupProxy) FetchTradingSpecifications(
	executionContext context.Context,
) ([]vo.ContractTradingSpecificationVo, error) {
	const everyContract = "every contract"

	exchangeInfo, catalogueError := binanceContractSymbolLookupProxy.fetchCatalogue(
		executionContext, everyContract)
	if catalogueError != nil {
		return nil, catalogueError
	}

	fundingIntervals, intervalError := binanceContractSymbolLookupProxy.fetchFundingIntervals(
		executionContext, everyContract)
	if intervalError != nil {
		return nil, intervalError
	}

	specifications := make([]vo.ContractTradingSpecificationVo, 0, len(exchangeInfo.Symbols))
	for _, listing := range exchangeInfo.Symbols {
		if !listing.isFollowable() {
			continue
		}

		// Skip one unreadable listing rather than failing all; absent means keep the last confirmed specification.
		specification, specificationError := listing.toContractTradingSpecificationVo(fundingIntervals)
		if specificationError != nil {
			continue
		}
		specifications = append(specifications, specification)
	}

	return specifications, nil
}

// fetchCatalogue reads the whole catalogue; subject only labels the error.
func (binanceContractSymbolLookupProxy *BinanceContractSymbolLookupProxy) fetchCatalogue(
	executionContext context.Context, subject string,
) (binanceContractExchangeInfo, error) {
	answer, askError := binanceContractSymbolLookupProxy.ask(
		executionContext, binanceContractSymbolLookupProxy.baseUrl, subject)
	if askError != nil {
		return binanceContractExchangeInfo{}, askError
	}

	var exchangeInfo binanceContractExchangeInfo
	if decodeError := json.Unmarshal(answer, &exchangeInfo); decodeError != nil {
		return binanceContractExchangeInfo{}, fmt.Errorf(
			"read contract market source answer for %s: %w", subject, decodeError)
	}

	return exchangeInfo, nil
}

func (binanceContractSymbolLookupProxy *BinanceContractSymbolLookupProxy) fetchFundingIntervals(
	executionContext context.Context, subject string,
) (map[string]int, error) {
	answer, askError := binanceContractSymbolLookupProxy.ask(
		executionContext, binanceContractSymbolLookupProxy.fundingInfoUrl, subject)
	if askError != nil {
		return nil, askError
	}

	var fundingIntervals []binanceFundingInterval
	if decodeError := json.Unmarshal(answer, &fundingIntervals); decodeError != nil {
		return nil, fmt.Errorf("read contract funding interval answer for %s: %w", subject, decodeError)
	}

	intervalsBySymbol := make(map[string]int, len(fundingIntervals))
	for _, fundingInterval := range fundingIntervals {
		intervalsBySymbol[fundingInterval.Symbol] = fundingInterval.FundingIntervalHours
	}

	return intervalsBySymbol, nil
}

// ask is separate so each response body (the catalogue is about a megabyte) is closed promptly.
func (binanceContractSymbolLookupProxy *BinanceContractSymbolLookupProxy) ask(
	executionContext context.Context, address string, subject string,
) ([]byte, error) {
	if waitError := binanceContractSymbolLookupProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet, address, nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach contract market source for %s: %w", subject, buildError)
	}

	response, requestError := binanceContractSymbolLookupProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach contract market source for %s: %w", subject, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"contract market source answered %d for %s", response.StatusCode, subject)
	}

	answer, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, fmt.Errorf("read contract market source answer for %s: %w", subject, readError)
	}

	return answer, nil
}

func (listing binanceContractListing) isFollowable() bool {
	return listing.Status == tradingStatus && perpetualContractTypes[listing.ContractType]
}

// toContractTradingSpecificationVo leaves the interval absent for contracts not in the funding interval list; the domain decides what that means.
func (listing binanceContractListing) toContractTradingSpecificationVo(
	fundingIntervals map[string]int,
) (vo.ContractTradingSpecificationVo, error) {
	// A missing filter leaves its figures empty so parsing fails rather than reading as zero.
	quotedTickSize, quotedStepSize, quotedMinimumQuantity, quotedMinimumNotional := "", "", "", ""
	for _, filter := range listing.Filters {
		switch filter.FilterType {
		case "PRICE_FILTER":
			quotedTickSize = filter.TickSize
		case "LOT_SIZE":
			quotedStepSize, quotedMinimumQuantity = filter.StepSize, filter.MinQty
		case "MIN_NOTIONAL":
			quotedMinimumNotional = filter.Notional
		}
	}

	specification := vo.ContractTradingSpecificationVo{Symbol: listing.Symbol}
	maintenanceMarginPercent := decimal.Zero
	for _, quotedFigure := range []struct {
		name   string
		quoted string
		target *decimal.Decimal
	}{
		{"tickSize", quotedTickSize, &specification.TickSize},
		{"stepSize", quotedStepSize, &specification.QuantityStep},
		{"minQty", quotedMinimumQuantity, &specification.MinimumQuantity},
		{"notional", quotedMinimumNotional, &specification.MinimumNotional},
		{"maintMarginPercent", listing.MaintMarginPercent, &maintenanceMarginPercent},
		{"liquidationFee", listing.LiquidationFee, &specification.LiquidationFeeRate},
	} {
		figure, parseError := decimal.NewFromString(quotedFigure.quoted)
		if parseError != nil {
			return vo.ContractTradingSpecificationVo{}, fmt.Errorf(
				"read %s of %s from contract market source: %w", quotedFigure.name, listing.Symbol, parseError)
		}
		*quotedFigure.target = figure
	}
	specification.MaintenanceMarginRate = maintenanceMarginPercent.Div(maintenanceMarginPercentScale)

	if fundingIntervalHours, isListed := fundingIntervals[listing.Symbol]; isListed {
		specification.FundingIntervalHours = &fundingIntervalHours
	}

	return specification, nil
}
