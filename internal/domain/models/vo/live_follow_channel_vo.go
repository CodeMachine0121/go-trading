package vo

import (
	"slices"
	"strings"
)

// LiveFollowChannelVo is one live channel for a market and a set of symbols (plans limit channels and symbols per channel); Key is the sorted set, so a changed set is a different channel.
type LiveFollowChannelVo struct {
	Market  MarketVo
	Symbols []string
	Key     string
}

// NewLiveFollowChannelVo is the only constructor, so Key always matches Symbols.
func NewLiveFollowChannelVo(market MarketVo, symbols []string) LiveFollowChannelVo {
	sortedSymbols := slices.Clone(symbols)
	slices.Sort(sortedSymbols)

	return LiveFollowChannelVo{
		Market:  market,
		Symbols: sortedSymbols,
		Key:     string(market) + "|" + strings.Join(sortedSymbols, ","),
	}
}
