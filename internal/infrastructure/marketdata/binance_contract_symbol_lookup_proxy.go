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

// tradingStatus is the only listing status whose candles will ever arrive. The venue
// also publishes contracts it has stopped trading (SETTLING) and ones it has not
// started (PENDING_TRADING), and both answer every fetch with nothing at all.
const tradingStatus = "TRADING"

// perpetualContractTypes are the venue's two kinds of contract that never deliver:
// the crypto ones and the traditional-finance ones (gold, silver, a share). Both
// carry a delivery date a century out, which is how the venue spells "no delivery".
//
// A dated future — CURRENT_QUARTER, NEXT_QUARTER, and whatever the venue adds next —
// is not one of these, and is deliberately refused rather than followed: a watchlist
// whose whole subject is 永續合約 must not quietly fill up with things that expire.
//
// **A kind this list does not recognise is refused, not assumed.** Refusing a new
// perpetual kind is a sentence somebody reads and one line to fix; following a new
// dated kind is a contract that silently stops producing candles on its delivery day.
var perpetualContractTypes = map[string]bool{
	"PERPETUAL":         true,
	"TRADIFI_PERPETUAL": true,
}

// maintenanceMarginPercentScale turns the venue's maintenance margin, which it writes
// as a percentage ("2.5000"), into the proportion the rest of the system speaks in.
// The liquidation fee it already writes as a proportion ("0.012500").
var maintenanceMarginPercentScale = decimal.NewFromInt(100)

// binanceContractFilter is one of the rules the venue attaches to a contract. Each
// kind carries its own fields, and a field a kind does not carry arrives empty.
type binanceContractFilter struct {
	FilterType string `json:"filterType"`
	TickSize   string `json:"tickSize"`
	StepSize   string `json:"stepSize"`
	MinQty     string `json:"minQty"`
	Notional   string `json:"notional"`
}

// binanceContractListing is one contract in the venue's catalogue, as far as this
// proxy reads it.
type binanceContractListing struct {
	Symbol             string                  `json:"symbol"`
	Status             string                  `json:"status"`
	ContractType       string                  `json:"contractType"`
	MaintMarginPercent string                  `json:"maintMarginPercent"`
	LiquidationFee     string                  `json:"liquidationFee"`
	Filters            []binanceContractFilter `json:"filters"`
}

// binanceContractExchangeInfo is the part of the venue's catalogue answer this needs.
//
// It reads fields the spot catalogue never had to: this venue answers with its whole
// listing whatever it is asked, so whether a contract is tradable, whether it is
// perpetual, and how trading it looks are all already in hand — and without the first
// two the only question this proxy can answer is "is that string in the file", which
// a delisted contract and a quarterly future both pass.
type binanceContractExchangeInfo struct {
	Symbols []binanceContractListing `json:"symbols"`
}

// binanceFundingInterval is one entry of the venue's list of contracts whose funding
// settles on an interval of their own.
type binanceFundingInterval struct {
	Symbol               string `json:"symbol"`
	FundingIntervalHours int    `json:"fundingIntervalHours"`
}

// BinanceContractSymbolLookupProxy answers whether Binance lists a perpetual
// contract that can actually be followed, and how trading each one looks.
//
// It cannot be the spot lookup pointed at another address, and that is a fact about
// the venue rather than a preference. The spot catalogue narrows to the pair it is
// asked about and refuses an unknown one, which is what lets the spot lookup read a
// refusal as "no such symbol". **The contract catalogue ignores the question
// entirely** — asked about a pair that does not exist, it answers with the whole
// catalogue and a perfectly ordinary success. A lookup written against the spot
// behaviour would therefore call every typo a real contract.
//
// So the whole catalogue is fetched and scanned. That costs about a megabyte per
// question, which is affordable because it is asked when somebody adds a contract to
// the watchlist and once a day to refresh the specifications — and having the whole
// catalogue is what lets the scan also refuse a contract that is listed but not
// followable.
//
// **A specification takes two answers.** The catalogue says how finely a contract
// trades; how often it settles its funding rate lives in a second list that names
// only the contracts with a setting of their own.
type BinanceContractSymbolLookupProxy struct {
	baseUrl        string
	fundingInfoUrl string
	httpClient     *http.Client
	// pacer is the contract venue's, shared with every other proxy that reaches it.
	// The allowance is counted per venue rather than per kind of question.
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

// LookUpSymbol reports whether the contract venue lists the symbol as a perpetual
// contract that is trading, and when it does, how trading it looks.
//
// **Being in the catalogue is not enough**, and the difference is not cosmetic. A
// contract the venue has stopped trading stays listed for a while and answers every
// fetch with nothing — so following one would put a symbol on the watchlist that is
// fetched every minute forever and stores nothing, which on this venue is
// indistinguishable from a quiet market because a perpetual contract is never
// presumed shut. A dated future would do the same on its delivery day.
//
// It never carries a display name. A pair on this venue is already its own name, and
// putting a translated one on screen would label it with something the venue has
// never used.
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

		// The funding interval list not answering does not make the contract
		// unfollowable either — the catalogue already said it is. Without the list the
		// interval cannot be known, and guessing eight hours for a contract that settles
		// every four would be wrong for a day, so no specification is handed on at all
		// and the daily refresh records it.
		fundingIntervals, intervalError := binanceContractSymbolLookupProxy.fetchFundingIntervals(
			executionContext, symbol)
		if intervalError != nil {
			return vo.ContractSymbolListingVo{IsListed: true}, nil
		}

		// A specification spelled in a way this cannot read does not make the contract
		// unfollowable. It is handed on empty, which is no specification at all, and
		// the daily refresh tries again.
		specification, specificationError := listing.toContractTradingSpecificationVo(fundingIntervals)
		if specificationError != nil {
			return vo.ContractSymbolListingVo{IsListed: true}, nil
		}

		return vo.ContractSymbolListingVo{IsListed: true, Specification: specification}, nil
	}

	return vo.ContractSymbolListingVo{}, nil
}

// FetchTradingSpecifications reports the specification of every contract the venue
// lists as a perpetual that is trading. A contract it no longer lists that way — or
// lists in a form this cannot read — is simply absent from the answer.
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

		// One listing the venue spelled in a way this cannot read is left out rather
		// than failing the whole answer: absent means "keep what was confirmed last",
		// and every other contract's refresh should not wait on one odd entry.
		specification, specificationError := listing.toContractTradingSpecificationVo(fundingIntervals)
		if specificationError != nil {
			continue
		}
		specifications = append(specifications, specification)
	}

	return specifications, nil
}

// fetchCatalogue reads the venue's whole catalogue. subject is only what the error
// names when it cannot.
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

// fetchFundingIntervals reads the venue's list of contracts whose funding settles on
// an interval of their own, keyed by symbol.
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

// ask puts one question to the venue and hands the answer back as it arrived.
//
// **It stays a method of its own because of what it encloses: one answer's body, from
// the moment it arrives to the moment it is let go** — and the catalogue is a
// megabyte of it.
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

// isFollowable says whether this listing is a perpetual that is trading.
func (listing binanceContractListing) isFollowable() bool {
	return listing.Status == tradingStatus && perpetualContractTypes[listing.ContractType]
}

// toContractTradingSpecificationVo reads this listing's specification. A contract the
// funding interval list does not name is handed on with no interval at all, rather
// than a guessed one: what that silence means is the domain's to say.
func (listing binanceContractListing) toContractTradingSpecificationVo(
	fundingIntervals map[string]int,
) (vo.ContractTradingSpecificationVo, error) {
	// Each figure lives in the filter of its own kind; a kind the listing lacks leaves
	// its figures empty, which fails to read below rather than passing as zero.
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
