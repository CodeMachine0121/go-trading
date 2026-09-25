package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_live_market_data_proxy.go -destination=mocks/mock_i_live_market_data_proxy.go -package=mocks

// ILiveMarketDataProxy follows a channel of symbols (the unit data plans are sold in) and streams each symbol's restated candle; closing the channel is the only end-of-feed signal.
// It makes one attempt and never reconnects, so retry and give-up rules stay testable in the domain.
type ILiveMarketDataProxy interface {
	FollowKCandles(
		executionContext context.Context, channel vo.LiveFollowChannelVo,
	) (<-chan vo.LiveKCandleVo, error)
}
