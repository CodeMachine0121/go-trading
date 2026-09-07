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

// fugleTicker is the part of this source's answer worth keeping: the company's own
// name. Everything else it reports about a stock — reference prices, day-trade
// eligibility, what industry it is in — belongs to a different feature than "is this
// code real and what is it called".
type fugleTicker struct {
	Name string `json:"name"`
}

// LookUpSymbol reports whether this source lists the symbol, and what it calls it.
//
// The name is read from the same answer that proves the code real, because it is in
// there. Asking again later would be a second round trip for something already on
// the desk — and would leave a window in which the two answers could disagree.
//
// A body this source will not let us read is not a reason to refuse the symbol: it
// said the code exists, and that is the question that decides whether somebody may
// watch it. The name is worth having and worth going without.
//
// The market is accepted and ignored: this proxy is only ever reached for the one
// market it serves, and taking the argument is what lets it satisfy the same contract
// every other source does.
func (fugleSymbolLookupProxy *FugleSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
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

	// This source says "no such symbol" by not finding it, which is an answer about
	// the symbol rather than a failure to answer.
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
