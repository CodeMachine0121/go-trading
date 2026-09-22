package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// SignalDomain is one candle's opinion, read out of a signal-kind script result:
// buy, sell or hold. The script states it through the values the runner injects
// (indicator.Buy / Sell / Hold), so by the time a result reaches here it already
// carries one of the three — the runner rejects anything else as a script failure
// before a replay ever sees it.
//
// It also says what it asks of the account holding it, and what the person reading a
// message about it has to go and do. Those two used to be a trading mode's answer,
// back when a sell meant "face the other way" to one set of rules and "get out into
// cash" to another. With one set of rules left, they are simply what the opinion
// means — "buy" is "I want to be holding this", and nothing has to be asked about the
// account first.
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

// TargetPosition is what this opinion asks the account to be holding once the candle
// is over.
//
// Buying asks to be holding the thing; selling asks to be back in cash; holding asks
// for nothing at all. Cash and no opinion are deliberately two different answers —
// collapsing them would turn "get out" into "leave it alone", which is invisible in
// every report card it ruins.
//
// An opinion this does not recognise asks for nothing. It is the loudest of the safe
// answers: a replay makes no trades at all, where reading it as cash would quietly
// close positions nobody meant to close and produce a page that looks entirely
// plausible.
func (signalDomain SignalDomain) TargetPosition() vo.TargetPositionVo {
	switch signalDomain.value {
	case vo.SignalBuy:
		return vo.TargetPositionLong
	case vo.SignalSell:
		return vo.TargetPositionFlat
	}

	return vo.TargetPositionUnchanged
}

// HeadlineVerb is what a message about this opinion asks its reader to go and do.
//
// A conclusion is read as an instruction, and the signal's own word is not always one
// the reader can carry out. Told 賣出, they ask what they are meant to be selling —
// they are flat, which is most of the time, because a sell reaches them on the
// strength of the signal alone and the signal has never known what they hold. 出場 is
// the one wording that reader can act on either way: there is a position, or there is
// not.
//
// Buying needs no such translation. 買入 is handing money over for a thing, with no
// second reading and nothing about the account left to find out — which is why the
// message carries no line naming the rules being traded by. There is one set of them,
// and this word already says which.
//
// Only the conclusion speaks of acts. The source lines below it keep quoting the
// scripts in 買入／賣出／持有, which is how a reader works back from the conclusion —
// see InWords.
func (signalDomain SignalDomain) HeadlineVerb() string {
	if signalDomain.value == vo.SignalSell {
		return "出場"
	}

	return signalDomain.InWords()
}
