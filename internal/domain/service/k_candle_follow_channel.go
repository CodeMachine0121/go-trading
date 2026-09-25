package service

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleFollowChannel is one live line and the symbols on it, which break, stall and recover together; its identity is the exact symbol set, so a changed set means a new channel.
type kCandleFollowChannel struct {
	channel  vo.LiveFollowChannelVo
	follows  map[string]*kCandleFollowSymbol
	cancel   context.CancelFunc
	finished chan struct{}
}

func newKCandleFollowChannel(
	channel vo.LiveFollowChannelVo,
	follows map[string]*kCandleFollowSymbol,
	cancel context.CancelFunc,
) *kCandleFollowChannel {
	return &kCandleFollowChannel{
		channel:  channel,
		follows:  follows,
		cancel:   cancel,
		finished: make(chan struct{}),
	}
}

// followOf returns the symbol's registry, or false for a symbol nobody subscribed to on this channel.
func (followChannel *kCandleFollowChannel) followOf(
	symbol string,
) (*kCandleFollowSymbol, bool) {
	follow, isCarried := followChannel.follows[symbol]

	return follow, isCarried
}

// isRostered reports whether the channel exists because of a market roster rather than a viewer; only those are the roster's to retire.
func (followChannel *kCandleFollowChannel) isRostered() bool {
	for _, follow := range followChannel.follows {
		if follow.isOnARoster {
			return true
		}
	}

	return false
}

// publishStalled tells every symbol on the channel that live updates stopped, skipping retired ones, which are owed their real reason instead.
func (followChannel *kCandleFollowChannel) publishStalled(
	retiredSymbols map[string]bool,
) {
	for symbol, follow := range followChannel.follows {
		if retiredSymbols[symbol] {
			continue
		}

		follow.publishStalled()
	}
}

// end stops the line and waits until nothing is publishing; whether its symbols are finished is left to the caller.
func (followChannel *kCandleFollowChannel) end() {
	followChannel.cancel()
	<-followChannel.finished
}
