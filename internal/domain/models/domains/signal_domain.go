package domains

import (
	"math"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SignalIndicatorName is the one indicator name a replay reads as an opinion. Every
// other name a script produces is a number to look at, not an instruction to act on.
//
// It is a constant rather than a literal buried in the reading below because it is
// half of a contract with whoever writes the script: rename it here and every script
// ever written goes quiet, which is exactly the kind of change that should be one
// visible line.
const SignalIndicatorName = "signal"

// SignalDomain is one candle's opinion, read out of that candle's indicator result.
//
// The reading is by sign rather than by matching 1, -1 and 0 exactly. A signal is a
// direction, and by sign every number a script can produce means something: a
// strength of 0.5 buys, a strength of -2 sells, and there is no such thing as a
// legal-but-meaningless value that would have to fail a replay half way through.
//
// Two things read as flat, and both matter. A result that never named a signal reads
// as flat, which is what lets every script written before signals existed replay
// untouched as a run with no trades. And a signal that is not a finite number — zero
// divided by zero, or anything divided by zero — also reads as flat: an arithmetic
// accident is not an instruction to bet money.
type SignalDomain struct {
	value vo.SignalVo
}

// NewSignalDomain reads one candle's indicator result. It cannot fail: every possible
// result, including a missing name and a broken sum, maps to one of the three
// opinions.
func NewSignalDomain(indicatorValues map[string]vo.IndicatorValueVo) SignalDomain {
	indicatorValue, isNamed := indicatorValues[SignalIndicatorName]
	if !isNamed || len(indicatorValue.Numbers) == 0 {
		return SignalDomain{value: vo.SignalFlat}
	}

	signalStrength := indicatorValue.Numbers[0]
	if math.IsNaN(signalStrength) || math.IsInf(signalStrength, 0) {
		return SignalDomain{value: vo.SignalFlat}
	}

	if signalStrength > 0 {
		return SignalDomain{value: vo.SignalBuy}
	}
	if signalStrength < 0 {
		return SignalDomain{value: vo.SignalSell}
	}

	return SignalDomain{value: vo.SignalFlat}
}

func (signalDomain SignalDomain) Value() vo.SignalVo {
	return signalDomain.value
}

// WantedDirection is which way this opinion asks the account to face, and whether it
// asks for anything at all. Answering both in one call is what keeps the caller from
// asking "is it flat" and then asking again which way — two questions that can only
// ever be answered together.
func (signalDomain SignalDomain) WantedDirection() (vo.PositionDirectionVo, bool) {
	if signalDomain.value == vo.SignalBuy {
		return vo.PositionDirectionLong, true
	}
	if signalDomain.value == vo.SignalSell {
		return vo.PositionDirectionShort, true
	}

	return "", false
}
