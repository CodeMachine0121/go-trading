package marketdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// FugleSymbolLookupProxy answers whether Fugle lists a Taiwan stock.
//
// It asks for the symbol's own details rather than for its candles. A stock that
// exists but has not traded today answers with no candles, which is indistinguishable
// from a stock that does not exist — and refusing to watch a real company because it
// was quiet is worse than the typo this check exists to catch.
type FugleSymbolLookupProxy struct {
	tickerBaseUrl string
	apiKey        string
	httpClient    *http.Client
}

func NewFugleSymbolLookupProxy(
	tickerBaseUrl string, apiKey string, requestTimeout time.Duration,
) *FugleSymbolLookupProxy {
	return &FugleSymbolLookupProxy{
		tickerBaseUrl: tickerBaseUrl,
		apiKey:        apiKey,
		httpClient:    &http.Client{Timeout: requestTimeout},
	}
}

// SymbolExists reports whether this source lists the symbol.
//
// The market is accepted and ignored: this proxy is only ever reached for the one
// market it serves, and taking the argument is what lets it satisfy the same contract
// every other source does.
func (fugleSymbolLookupProxy *FugleSymbolLookupProxy) SymbolExists(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (bool, error) {
	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		fugleSymbolLookupProxy.tickerBaseUrl+"/"+url.PathEscape(symbol), nil)
	if buildError != nil {
		return false, fmt.Errorf("reach market source for %s: %w", symbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fugleSymbolLookupProxy.apiKey)

	response, requestError := fugleSymbolLookupProxy.httpClient.Do(request)
	if requestError != nil {
		return false, fmt.Errorf("reach market source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	// This source says "no such symbol" by not finding it, which is an answer about
	// the symbol rather than a failure to answer.
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("market source answered %d for %s", response.StatusCode, symbol)
	}

	return true, nil
}
