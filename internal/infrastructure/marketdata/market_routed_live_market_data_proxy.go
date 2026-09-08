package marketdata

import (
	"context"
	"fmt"
	"maps"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketRoutedLiveMarketDataProxy opens each live feed against the source that
// serves the market it belongs to.
//
// Like its fetching counterpart it satisfies the contract it routes to, so following
// a market live reads the same whether the system knows one venue or four.
type MarketRoutedLiveMarketDataProxy struct {
	liveMarketDataProxies map[vo.MarketVo]_interface.ILiveMarketDataProxy
}

func NewMarketRoutedLiveMarketDataProxy(
	liveMarketDataProxies map[vo.MarketVo]_interface.ILiveMarketDataProxy,
) *MarketRoutedLiveMarketDataProxy {
	return &MarketRoutedLiveMarketDataProxy{liveMarketDataProxies: maps.Clone(liveMarketDataProxies)}
}

// FollowKCandles opens one channel against the source that serves its market.
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
