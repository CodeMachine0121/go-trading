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

// FugleSymbolLookupProxy checks the symbol's ticker details rather than its candles, since a real stock that has not traded today returns no candles.
type FugleSymbolLookupProxy struct {
	tickerBaseUrl string
	apiKey        string
	httpClient    *http.Client
	// The pacer is shared with every proxy on the venue, since the allowance is per venue.
	pacer RequestPacer
}

func NewFugleSymbolLookupProxy(
	tickerBaseUrl string, apiKey string, requestTimeout time.Duration, pacer RequestPacer,
) *FugleSymbolLookupProxy {
	return &FugleSymbolLookupProxy{
		tickerBaseUrl: tickerBaseUrl,
		apiKey:        apiKey,
		httpClient:    &http.Client{Timeout: requestTimeout},
		pacer:         pacer,
	}
}

// fugleTicker keeps only the company name.
type fugleTicker struct {
	Name string `json:"name"`
}

// LookUpSymbol takes the display name from the same answer that proves the symbol exists; an unreadable body still counts as found, just without a name, and the market argument is ignored.
func (fugleSymbolLookupProxy *FugleSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
	if waitError := fugleSymbolLookupProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return vo.SymbolListingVo{}, waitError
	}

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		fugleSymbolLookupProxy.tickerBaseUrl+"/"+url.PathEscape(symbol), nil)
	if buildError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"reach market source for %s: %w", symbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fugleSymbolLookupProxy.apiKey)

	response, requestError := fugleSymbolLookupProxy.httpClient.Do(request)
	if requestError != nil {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	// 404 means "no such symbol", not a failure.
	if response.StatusCode == http.StatusNotFound {
		return vo.SymbolListingVo{}, nil
	}

	if response.StatusCode != http.StatusOK {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, symbol)
	}

	var ticker fugleTicker
	if decodeError := json.NewDecoder(response.Body).Decode(&ticker); decodeError != nil {
		return vo.SymbolListingVo{IsListed: true}, nil
	}

	return vo.SymbolListingVo{IsListed: true, DisplayName: ticker.Name}, nil
}
