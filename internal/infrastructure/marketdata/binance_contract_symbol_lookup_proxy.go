package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// BinanceContractSymbolLookupProxy answers whether Binance lists a perpetual
// contract.
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
// contract to the watchlist and at no other time.
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

// LookUpSymbol reports whether the contract venue lists the symbol.
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

	var exchangeInfo binanceExchangeInfo
	if decodeError := json.NewDecoder(response.Body).Decode(&exchangeInfo); decodeError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"read contract market source answer for %s: %w", symbol, decodeError)
	}

	for _, listedSymbol := range exchangeInfo.Symbols {
		if listedSymbol.Symbol == symbol {
			return vo.SymbolListingVo{IsListed: true}, nil
		}
	}

	return vo.SymbolListingVo{}, nil
}
