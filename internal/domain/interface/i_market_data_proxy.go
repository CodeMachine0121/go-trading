package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_market_data_proxy.go -destination=mocks/mock_i_market_data_proxy.go -package=mocks

// IMarketDataProxy fetches K candles from a market source. It is named for the
// capability, not the provider, so a different exchange is a new implementation
// rather than a new contract.
//
// One window is the only way to ask. Both the periodic round and the startup
// backfill are windows, so everything a source needs to hide — the address, the
// symbol spelling, the wire format, paging, timeouts — stays behind this method.
type IMarketDataProxy interface {
	FetchKCandles(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error)
}
