package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_live_market_data_proxy.go -destination=mocks/mock_i_live_market_data_proxy.go -package=mocks

// ILiveMarketDataProxy follows one market as it trades. It is named for the
// capability, not the provider, so a different exchange is a new implementation
// rather than a new contract.
//
// One attempt is all it promises. The channel it returns carries the symbol's
// candle as the source keeps restating it, and closing that channel is the only
// way it reports that the feed has ended — a caller never has to ask about state.
//
// It deliberately does not reconnect. How long to wait before trying again, and
// whether to give up, are rules the requirements state in so many words, so they
// belong where a table test can reach them rather than behind a socket.
type ILiveMarketDataProxy interface {
	FollowKCandles(
		executionContext context.Context, symbol string,
	) (<-chan vo.LiveKCandleVo, error)
}
