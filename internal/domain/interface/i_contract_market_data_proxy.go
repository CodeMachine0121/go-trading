package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_market_data_proxy.go -destination=mocks/mock_i_contract_market_data_proxy.go -package=mocks

// IContractMarketDataProxy hides that a contract candle merges traded figures with mark price (plus paging, pacing and placeholder zeros).
// A minute missing its mark price comes back with it absent rather than dropped, so the domain can reject it and record why; the window's market is ignored.
type IContractMarketDataProxy interface {
	FetchKCandles(
		executionContext context.Context, window vo.KCandleFetchWindowVo,
	) ([]vo.ContractMarketKCandleVo, error)
}
