package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

type MarketRoutedSymbolLookupProxy struct {
	symbolLookupProxies map[vo.MarketVo]_interface.ISymbolLookupProxy
}

func NewMarketRoutedSymbolLookupProxy(
	symbolLookupProxies map[vo.MarketVo]_interface.ISymbolLookupProxy,
) *MarketRoutedSymbolLookupProxy {
	return &MarketRoutedSymbolLookupProxy{symbolLookupProxies: maps.Clone(symbolLookupProxies)}
}

// LookUpSymbol fails for an unwired market rather than reporting "no such symbol", since the two mean very different things to the caller.
func (marketRoutedSymbolLookupProxy *MarketRoutedSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
	symbolLookupProxy, isServed := marketRoutedSymbolLookupProxy.symbolLookupProxies[market]
	if !isServed {
		return vo.SymbolListingVo{}, fmt.Errorf(
			"no market data source is wired up for %s", market)
	}

	return symbolLookupProxy.LookUpSymbol(executionContext, market, symbol)
}
