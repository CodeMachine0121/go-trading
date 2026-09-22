package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_market_data_proxy.go -destination=mocks/mock_i_contract_market_data_proxy.go -package=mocks

// IContractMarketDataProxy fetches perpetual contract K candles.
//
// One call, one answer. That a contract candle is assembled from two separate
// questions to the venue — the traded figures and the mark price — is this contract's
// to hide, along with the paging, the pacing, and the placeholder zeros the venue
// attaches to the second answer. A caller that had to ask twice and align the results
// would be a caller that could align them wrongly.
//
// What it does not hide is an incomplete result: a minute whose mark price did not
// arrive comes back with that figure absent rather than invented or dropped, because
// "a contract candle without a mark price is not one" is a rule, and rules belong to
// the domain. Dropping it here would also cost the reason, and the record kept of a
// skipped candle is supposed to say what was missing.
//
// The window carries a market, which this proxy does not read: the contract path has
// exactly one venue, so there is nothing to route on. It takes the same window type
// as the spot side because there is one notion of "a stretch to fetch" in this system
// and a second one would only ever be the same three fields.
type IContractMarketDataProxy interface {
	FetchKCandles(
		executionContext context.Context, window vo.KCandleFetchWindowVo,
	) ([]vo.ContractMarketKCandleVo, error)
}
