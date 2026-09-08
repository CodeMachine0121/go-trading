package service

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// followChannel is one live channel being kept open, and the trading symbols
// travelling on it.
//
// It exists because a channel is what actually breaks and comes back. The symbols on
// it go quiet together, are told together, and are waited for together — so the
// retry loop, the silence and the context that ends it all belong here rather than
// being repeated once per symbol.
//
// It is recognised by the channel it carries, and a channel is recognised by the
// exact set of symbols on it. That is what makes "rebuild when the followed set
// changes, leave it alone when it does not" a fact about identity rather than a
// comparison somebody has to write: a different set is a different key, and a key
// that is still there was never asked to change.
type followChannel struct {
	channel vo.LiveFollowChannelVo
	// follows are the per-symbol viewer registries this channel feeds, held so that a
	// candle arriving can be handed to the right one and an outage can be told to
	// every one of them.
	follows  map[string]*symbolFollow
	cancel   context.CancelFunc
	finished chan struct{}
}

func newFollowChannel(
	channel vo.LiveFollowChannelVo,
	follows map[string]*symbolFollow,
	cancel context.CancelFunc,
) *followChannel {
	return &followChannel{
		channel:  channel,
		follows:  follows,
		cancel:   cancel,
		finished: make(chan struct{}),
	}
}

// followOf is the viewer registry of one symbol on this channel, and whether this
// channel carries it at all. A source restating something nobody subscribed to
// belongs to nobody here.
func (followChannel *followChannel) followOf(symbol string) (*symbolFollow, bool) {
	follow, isCarried := followChannel.follows[symbol]

	return follow, isCarried
}

// isRostered reports a channel the system keeps up because a market's places were
// handed to its symbols, rather than because somebody is watching. Only those are
// the roster's to retire.
func (followChannel *followChannel) isRostered() bool {
	for _, follow := range followChannel.follows {
		if follow.isOnARoster {
			return true
		}
	}

	return false
}

// publishStalled tells every symbol on this channel that live updating has stopped.
// One line went down, so it is one piece of news — said to everyone it reaches.
func (followChannel *followChannel) publishStalled() {
	for _, follow := range followChannel.follows {
		follow.publishStalled()
	}
}

// end stops this channel and closes every viewer's updates on every symbol it
// carried, waiting for the work to finish first so that nothing is still publishing
// into a channel about to close.
func (followChannel *followChannel) end() {
	followChannel.cancel()
	<-followChannel.finished

	for _, follow := range followChannel.follows {
		follow.end()
	}
}
