package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// SignalDomain is one candle's opinion, read out of a signal-kind script result:
// buy, sell or hold. The script states it through the values the runner injects
// (indicator.Buy / Sell / Hold), so by the time a result reaches here it already
// carries one of the three — the runner rejects anything else as a script failure
// before a replay ever sees it.
//
// It stops at what the opinion says. What that opinion then asks an account to be
// holding is the trading mode's answer, not this one's — a sell means face the other
// way to one mode and get out into cash to the other, and a signal that answered it
// itself would be a second opinion nobody could overrule.
type SignalDomain struct {
	value vo.SignalVo
}

// NewSignalDomain reads the one signal out of a script result. Where in the result
// a signal-kind script files its signal is this model's knowledge, not the caller's:
// a caller handed the whole result should not have to know the key to reach past.
func NewSignalDomain(indicatorValues map[string]vo.IndicatorValueVo) SignalDomain {
	return SignalDomain{value: indicatorValues[vo.SignalIndicatorKey].Signal}
}

// NewSignalDomainOf is one opinion already in hand.
//
// Two callers have a signal and no script result to read it out of: a replay of a
// whole trading strategy, which has already combined several sources into one, and a
// bot's round, which concluded one a while ago. Both were building a one-entry map for
// the sake of the other constructor, and a map built to satisfy a reader is a shape
// nobody meant.
func NewSignalDomainOf(signal vo.SignalVo) SignalDomain {
	return SignalDomain{value: signal}
}

func (signalDomain SignalDomain) Value() vo.SignalVo {
	return signalDomain.value
}

// InWords is this opinion as a person reads it.
//
// It stays the signal's own vocabulary whichever mode is reading. What somebody has
// to go and do about one is the trading mode's business — a sell asks one account to
// open a position and another to close one — and rewriting these words to match that
// would put them in the strategy scripts' mouths, leaving a reader unable to work
// back from the conclusion to what produced it.
//
// An unrecognised value is written out as it stands rather than replaced with a
// guess: this is the last place to quietly turn something the system did not
// understand into one of the three things it did.
func (signalDomain SignalDomain) InWords() string {
	switch signalDomain.value {
	case vo.SignalBuy:
		return "買入"
	case vo.SignalSell:
		return "賣出"
	case vo.SignalHold:
		return "持有"
	}

	return string(signalDomain.value)
}
