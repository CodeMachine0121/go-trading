package vo

import (
	"slices"
	"strings"
)

// LiveFollowChannelVo is one live channel: which market it reaches, and which
// trading symbols travel on it.
//
// A channel carries a set rather than a single symbol because that is how the market
// data plans are sold — so many channels at once, so many symbols on each. Following
// four symbols down four channels when the plan allows one is how a system gets
// itself disconnected on every attempt.
//
// Key is what makes two channels the same channel, and it is the whole set: change
// which symbols travel on it and it is a different channel. That is deliberate. It
// makes "rebuild when the followed set changes, and leave it alone when it does not"
// something nothing has to implement — nothing compares two sets anywhere, because
// the identity already did.
//
// Immutable, no behavior. The symbols are sorted once here so that the same set
// always spells the same key, whatever order it arrived in.
type LiveFollowChannelVo struct {
	Market  MarketVo
	Symbols []string
	Key     string
}

// NewLiveFollowChannelVo is the only way one is built, so a channel can never exist
// with a key that does not match the symbols it carries.
func NewLiveFollowChannelVo(market MarketVo, symbols []string) LiveFollowChannelVo {
	sortedSymbols := slices.Clone(symbols)
	slices.Sort(sortedSymbols)

	return LiveFollowChannelVo{
		Market:  market,
		Symbols: sortedSymbols,
		Key:     string(market) + "|" + strings.Join(sortedSymbols, ","),
	}
}
