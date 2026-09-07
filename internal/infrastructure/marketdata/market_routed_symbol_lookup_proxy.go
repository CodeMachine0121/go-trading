package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketRoutedSymbolLookupProxy asks the source that serves a market whether it has
// heard of a symbol.
type MarketRoutedSymbolLookupProxy struct {
	symbolLookupProxies map[vo.MarketVo]_interface.ISymbolLookupProxy
}

func NewMarketRoutedSymbolLookupProxy(
	symbolLookupProxies map[vo.MarketVo]_interface.ISymbolLookupProxy,
) *MarketRoutedSymbolLookupProxy {
	return &MarketRoutedSymbolLookupProxy{symbolLookupProxies: maps.Clone(symbolLookupProxies)}
}

// SymbolExists asks the market itself.
//
// A market with nothing wired up is a failure rather than "no such symbol". The two
// mean opposite things to whoever asked — fix what you typed, versus this system is
// not finished — and answering the wrong one sends them looking in the wrong place.
func (marketRoutedSymbolLookupProxy *MarketRoutedSymbolLookupProxy) SymbolExists(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (bool, error) {
	symbolLookupProxy, isServed := marketRoutedSymbolLookupProxy.symbolLookupProxies[market]
	if !isServed {
		return false, fmt.Errorf("no market data source is wired up for %s", market)
	}

	return symbolLookupProxy.SymbolExists(executionContext, market, symbol)
}
