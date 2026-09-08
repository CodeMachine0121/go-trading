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
// It is asked for a channel — a set of trading symbols to follow down one line —
// rather than for a single symbol, because that is the unit the market data plans
// are sold in. How many symbols one line may carry is a fact about the venue and
// lives with the rest of them; this contract only has to be able to express it.
//
// One attempt is all it promises. The channel it returns carries each followed
// symbol's candle as the source keeps restating it, every candle naming the symbol
// it belongs to, and closing that channel is the only way it reports that the feed
// has ended — a caller never has to ask about state.
//
// It deliberately does not reconnect. How long to wait before trying again, and
// whether to give up, are rules the requirements state in so many words, so they
// belong where a table test can reach them rather than behind a socket.
type ILiveMarketDataProxy interface {
	FollowKCandles(
		executionContext context.Context, channel vo.LiveFollowChannelVo,
	) (<-chan vo.LiveKCandleVo, error)
}
