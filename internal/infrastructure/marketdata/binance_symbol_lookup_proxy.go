package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// binanceExchangeInfo is the part of the source's catalogue answer this needs: which
// of the symbols asked about it actually lists.
type binanceExchangeInfo struct {
	Symbols []struct {
		Symbol string `json:"symbol"`
	} `json:"symbols"`
}

// BinanceSymbolLookupProxy answers whether Binance lists a trading pair.
//
// It asks the source's catalogue rather than trying to fetch a candle. A pair that
// exists but has not traded in the window asked for answers with no candles, which is
// indistinguishable from a pair that does not exist — and refusing to add a real
// market because it was quiet is worse than the typo this check exists to catch.
type BinanceSymbolLookupProxy struct {
	baseUrl    string
	httpClient *http.Client
}

func NewBinanceSymbolLookupProxy(baseUrl string, requestTimeout time.Duration) *BinanceSymbolLookupProxy {
	return &BinanceSymbolLookupProxy{
		baseUrl:    baseUrl,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// LookUpSymbol reports whether this source lists the symbol.
//
// It never carries a display name. A pair on this venue is already its own name, and
// putting a translated one on screen would label it with something the venue has
// never used.
//
// The market is accepted and ignored: this proxy is only ever reached for the one
// market it serves, and taking the argument is what lets it satisfy the same contract
// every other source does.
func (binanceSymbolLookupProxy *BinanceSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
	queryValues := url.Values{}
	queryValues.Set("symbol", symbol)

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		binanceSymbolLookupProxy.baseUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf("reach market source for %s: %w", symbol, buildError)
	}

	response, requestError := binanceSymbolLookupProxy.httpClient.Do(request)
	if requestError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf("reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	// This source refuses an unknown pair rather than answering with an empty
	// catalogue, so a refusal is the answer "no such symbol" and not a failure.
	if response.StatusCode == http.StatusBadRequest {
		return vo.SymbolListingVo{}, nil
	}

	if response.StatusCode != http.StatusOK {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, symbol)
	}

	var exchangeInfo binanceExchangeInfo
	if decodeError := json.NewDecoder(response.Body).Decode(&exchangeInfo); decodeError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"read market source answer for %s: %w", symbol, decodeError)
	}

	for _, listedSymbol := range exchangeInfo.Symbols {
		if listedSymbol.Symbol == symbol {
			return vo.SymbolListingVo{IsListed: true}, nil
		}
	}

	return vo.SymbolListingVo{}, nil
}
