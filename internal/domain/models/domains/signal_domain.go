package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// SignalDomain is one candle's opinion, read out of a signal-kind script result:
// buy, sell or hold. The script states it through the values the runner injects
// (indicator.Buy / Sell / Hold), so by the time a result reaches here it already
// carries one of the three — the runner rejects anything else as a script failure
// before a replay ever sees it.
//
// Two things a replay needs to know about an opinion can only be answered together —
// whether it asks for a position at all, and which way — so WantedDirection answers
// both in one call.
type SignalDomain struct {
	value vo.SignalVo
}

// NewSignalDomain reads the one signal out of a script result. Where in the result
// a signal-kind script files its signal is this model's knowledge, not the caller's:
// a caller handed the whole result should not have to know the key to reach past.
func NewSignalDomain(indicatorValues map[string]vo.IndicatorValueVo) SignalDomain {
	return SignalDomain{value: indicatorValues[vo.SignalIndicatorKey].Signal}
}

func (signalDomain SignalDomain) Value() vo.SignalVo {
	return signalDomain.value
}

// WantedDirection is which way this opinion asks the account to face, and whether it
// asks for anything at all. Hold asks for nothing; buy and sell ask for the two
// directions. Answering both in one call is what keeps the caller from asking "is it
// hold" and then asking again which way — two questions that can only ever be
// answered together.
func (signalDomain SignalDomain) WantedDirection() (vo.PositionDirectionVo, bool) {
	if signalDomain.value == vo.SignalBuy {
		return vo.PositionDirectionLong, true
	}
	if signalDomain.value == vo.SignalSell {
		return vo.PositionDirectionShort, true
	}

	return "", false
}
