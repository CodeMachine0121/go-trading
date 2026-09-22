package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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

// binanceContractExchangeInfo is the part of the venue's catalogue answer this needs.
//
// It reads two fields the spot catalogue never had to: this venue answers with its
// whole listing whatever it is asked, so the fields that say whether a contract is
// tradable and whether it is perpetual are already in hand — and without them the
// only question this proxy can answer is "is that string in the file", which a
// delisted contract and a quarterly future both pass.
type binanceContractExchangeInfo struct {
	Symbols []struct {
		Symbol       string `json:"symbol"`
		Status       string `json:"status"`
		ContractType string `json:"contractType"`
	} `json:"symbols"`
}

// BinanceContractSymbolLookupProxy answers whether Binance lists a perpetual
// contract that can actually be followed.
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
// question, which is affordable because the question is asked when somebody adds a
// contract to the watchlist and at no other time — and having the whole catalogue is
// what lets the scan also refuse a contract that is listed but not followable.
type BinanceContractSymbolLookupProxy struct {
	baseUrl    string
	httpClient *http.Client
	// pacer is the contract venue's, shared with every other proxy that reaches it.
	// The allowance is counted per venue rather than per kind of question.
	pacer RequestPacer
}

func NewBinanceContractSymbolLookupProxy(
	baseUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractSymbolLookupProxy {
	return &BinanceContractSymbolLookupProxy{
		baseUrl:    baseUrl,
		httpClient: &http.Client{Timeout: requestTimeout},
		pacer:      pacer,
	}
}

// LookUpSymbol reports whether the contract venue lists the symbol as a perpetual
// contract that is trading.
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
) (vo.SymbolListingVo, error) {
	if waitError := binanceContractSymbolLookupProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return vo.SymbolListingVo{}, waitError
	}

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		binanceContractSymbolLookupProxy.baseUrl, nil)
	if buildError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"reach contract market source for %s: %w", symbol, buildError)
	}

	response, requestError := binanceContractSymbolLookupProxy.httpClient.Do(request)
	if requestError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"reach contract market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"contract market source answered %d for %s", response.StatusCode, symbol)
	}

	var exchangeInfo binanceContractExchangeInfo
	if decodeError := json.NewDecoder(response.Body).Decode(&exchangeInfo); decodeError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"read contract market source answer for %s: %w", symbol, decodeError)
	}

	for _, listedSymbol := range exchangeInfo.Symbols {
		if listedSymbol.Symbol != symbol {
			continue
		}

		isFollowable := listedSymbol.Status == tradingStatus &&
			perpetualContractTypes[listedSymbol.ContractType]

		return vo.SymbolListingVo{IsListed: isFollowable}, nil
	}

	return vo.SymbolListingVo{}, nil
}
