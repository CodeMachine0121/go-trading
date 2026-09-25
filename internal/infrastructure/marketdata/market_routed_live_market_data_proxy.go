package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketRoutedLiveMarketDataProxy routes each live feed to the source serving its market.
type MarketRoutedLiveMarketDataProxy struct {
	liveMarketDataProxies map[vo.MarketVo]_interface.ILiveMarketDataProxy
}

func NewMarketRoutedLiveMarketDataProxy(
	liveMarketDataProxies map[vo.MarketVo]_interface.ILiveMarketDataProxy,
) *MarketRoutedLiveMarketDataProxy {
	return &MarketRoutedLiveMarketDataProxy{liveMarketDataProxies: maps.Clone(liveMarketDataProxies)}
}

func (marketRoutedLiveMarketDataProxy *MarketRoutedLiveMarketDataProxy) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	liveMarketDataProxy, isServed := marketRoutedLiveMarketDataProxy.
		liveMarketDataProxies[channel.Market]
	if !isServed {
		return nil, fmt.Errorf("no live market data source is wired up for %s", channel.Market)
	}

	return liveMarketDataProxy.FollowKCandles(executionContext, channel)
}
