package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketRoutedMarketDataProxy sends each request to the source that serves the
// market it names.
//
// It satisfies the very contract it routes to, which is the point: nothing above it
// learns that more than one source exists. A domain service asks for a window of
// candles exactly as it did when there was one exchange, and adding a third market
// is one more entry in the map handed in here — no interface changes, no service
// changes, and no branch anywhere asking which market it is looking at.
//
// The routing dimension is the market rather than the transport, which is why the
// name says so. Two sources that both speak HTTP are still two markets.
type MarketRoutedMarketDataProxy struct {
	marketDataProxies map[vo.MarketVo]_interface.IMarketDataProxy
}

func NewMarketRoutedMarketDataProxy(
	marketDataProxies map[vo.MarketVo]_interface.IMarketDataProxy,
) *MarketRoutedMarketDataProxy {
	return &MarketRoutedMarketDataProxy{marketDataProxies: maps.Clone(marketDataProxies)}
}

// FetchKCandles hands the window to the source that serves its market.
//
// A market with no source is a failure rather than an empty answer. Empty would read
// as "the market had nothing", and a whole market quietly holding nothing is how a
// missing wire-up survives to production.
func (marketRoutedMarketDataProxy *MarketRoutedMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketDataProxy, isServed := marketRoutedMarketDataProxy.marketDataProxies[window.Market]
	if !isServed {
		return nil, fmt.Errorf("no market data source is wired up for %s", window.Market)
	}

	return marketDataProxy.FetchKCandles(executionContext, window)
}
