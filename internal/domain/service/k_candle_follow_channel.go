package service

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleFollowChannel is one live channel being kept open, and the trading symbols
// travelling on it.
//
// It exists because a channel is what actually breaks and comes back. The symbols on
// it go quiet together, are told together, and are waited for together — so the
// retry loop, the silence and the context that ends it all belong here rather than
// being repeated once per symbol.
//
// It carries execution, not domain data — a cancel function, a done signal, and the
// registries it feeds — so it is not an entity, a domain model or a value object and
// has no place under models/. It lives beside the service that runs it and takes no
// suffix, because a suffix would invite somebody to move it and give it rules. The
// rules it needs are asked of LiveChannelHealthDomain instead.
//
// It is recognised by the channel it carries, and a channel is recognised by the
// exact set of symbols on it. That is what makes "rebuild when the followed set
// changes, leave it alone when it does not" a fact about identity rather than a
// comparison somebody has to write: a different set is a different key, and a key
// that is still there was never asked to change.
type kCandleFollowChannel struct {
	channel vo.LiveFollowChannelVo
	// follows are the per-symbol viewer registries this channel feeds, held so that a
	// candle arriving can be handed to the right one and an outage can be told to
	// every one of them.
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

// followOf is the viewer registry of one symbol on this channel, and whether this
// channel carries it at all. A source restating something nobody subscribed to
// belongs to nobody here.
func (followChannel *kCandleFollowChannel) followOf(
	symbol string,
) (*kCandleFollowSymbol, bool) {
	follow, isCarried := followChannel.follows[symbol]

	return follow, isCarried
}

// isRostered reports a channel the system keeps up because a market's places were
// handed to its symbols, rather than because somebody is watching. Only those are
// the roster's to retire.
func (followChannel *kCandleFollowChannel) isRostered() bool {
	for _, follow := range followChannel.follows {
		if follow.isOnARoster {
			return true
		}
	}

	return false
}

// publishStalled tells the symbols on this channel that live updating has stopped.
// One line went down, so it is one piece of news — said to everyone it reaches.
//
// Except those the caller names as retired. "Stalled" promises the picture is coming
// back, and a symbol whose place has gone is owed the truer reason instead; hearing
// the promise first and the truth a moment later is two answers to one question.
// A caller with nobody to leave out passes nothing.
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

// end stops this channel and waits for the work to finish, so that nothing is still
// publishing by the time it returns.
//
// It stops the line and nothing else. Whether the symbols that travelled on it are
// finished with is a separate question with a separate answer — a line replaced
// because the roster changed carries symbols that are still very much being followed
// — and it is answered by whoever asked for the line to end.
func (followChannel *kCandleFollowChannel) end() {
	followChannel.cancel()
	<-followChannel.finished
}
