package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketRoutedMarketDataProxy routes each request to the source serving its market, implementing the same interface so callers never see multiple sources.
type MarketRoutedMarketDataProxy struct {
	marketDataProxies map[vo.MarketVo]_interface.IMarketDataProxy
}

func NewMarketRoutedMarketDataProxy(
	marketDataProxies map[vo.MarketVo]_interface.IMarketDataProxy,
) *MarketRoutedMarketDataProxy {
	return &MarketRoutedMarketDataProxy{marketDataProxies: maps.Clone(marketDataProxies)}
}

// FetchKCandles fails for a market with no source rather than returning empty, so a missing wire-up is not mistaken for no data.
func (marketRoutedMarketDataProxy *MarketRoutedMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	marketDataProxy, isServed := marketRoutedMarketDataProxy.marketDataProxies[window.Market]
	if !isServed {
		return nil, fmt.Errorf("no market data source is wired up for %s", window.Market)
	}

	return marketDataProxy.FetchKCandles(executionContext, window)
}
