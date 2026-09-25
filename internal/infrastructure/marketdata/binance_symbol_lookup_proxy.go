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

type binanceExchangeInfo struct {
	Symbols []struct {
		Symbol string `json:"symbol"`
	} `json:"symbols"`
}

// BinanceSymbolLookupProxy checks the exchange catalogue rather than fetching candles, since a quiet real pair would return no candles just like a typo.
type BinanceSymbolLookupProxy struct {
	baseUrl    string
	httpClient *http.Client
	// The pacer is shared with every proxy on the venue, since the allowance is per venue.
	pacer RequestPacer
}

func NewBinanceSymbolLookupProxy(
	baseUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceSymbolLookupProxy {
	return &BinanceSymbolLookupProxy{
		baseUrl:    baseUrl,
		httpClient: &http.Client{Timeout: requestTimeout},
		pacer:      pacer,
	}
}

// LookUpSymbol never sets a display name, and ignores the market argument because this proxy serves only one market.
func (binanceSymbolLookupProxy *BinanceSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
	if waitError := binanceSymbolLookupProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return vo.SymbolListingVo{}, waitError
	}

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

	// The venue returns 400 for an unknown pair, which means "no such symbol", not a failure.
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
