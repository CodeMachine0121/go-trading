package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_market_data_proxy.go -destination=mocks/mock_i_market_data_proxy.go -package=mocks

// IMarketDataProxy fetches K candles for one window, hiding the source's address, symbol spelling, wire format, paging and timeouts.
type IMarketDataProxy interface {
	FetchKCandles(
		executionContext context.Context, window vo.KCandleFetchWindowVo,
	) ([]vo.MarketKCandleVo, error)
}
