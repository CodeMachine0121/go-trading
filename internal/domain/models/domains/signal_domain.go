package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// SignalDomain is one candle's opinion: buy, sell or hold. The script states it
// through the values the runner injects (indicator.Buy / Sell / Hold), so by the
// time one reaches here it is already one of the three — the runner rejects anything
// else as a script failure before a replay ever sees it.
//
// Two things a replay needs to know about an opinion can only be answered together —
// whether it asks for a position at all, and which way — so WantedDirection answers
// both in one call.
type SignalDomain struct {
	value vo.SignalVo
}

// NewSignalDomain wraps a signal the script runner has already accepted. It cannot
// fail: validation is the runner's job, done once, at the boundary where the script's
// raw output is read.
func NewSignalDomain(value vo.SignalVo) SignalDomain {
	return SignalDomain{value: value}
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
