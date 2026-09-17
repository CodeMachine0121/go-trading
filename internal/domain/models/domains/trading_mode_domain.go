package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// selectableTradingModes is the entire set a caller may declare, in the order it is
// offered back when a declaration is not recognised.
var selectableTradingModes = []vo.TradingModeVo{
	vo.TradingModeLongShort,
	vo.TradingModeSpot,
}

// TradingModeDomain is which set of rules a replay trades by, and the one question
// that difference comes down to: what does this candle's opinion ask the account to
// be holding?
//
// Its whole job is to answer that in a single value. Everything after the answer —
// is that already what is held, does something have to be closed first, can the
// account afford to open — is the same whichever mode asked, so it is written once in
// BacktestAccountDomain rather than once per mode.
//
// Its zero value is not a usable mode; it is only ever returned alongside an error.
type TradingModeDomain struct {
	value vo.TradingModeVo
}

// NewTradingModeDomain reads what the caller declared. Declaring nothing means always
// being in the market, which is what a replay did before there was anything to
// declare — a default's job is to leave every existing question answered as it was.
//
// A declaration that is not recognised is refused rather than quietly read as the
// default. Falling back would hand somebody a report card built by the other set of
// rules, and nothing about it would say so.
func NewTradingModeDomain(declaredMode string) (TradingModeDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declaredMode)
	if normalizedDeclaration == "" {
		return TradingModeDomain{value: vo.TradingModeLongShort}, nil
	}

	for _, selectableMode := range selectableTradingModes {
		if strings.EqualFold(string(selectableMode), normalizedDeclaration) {
			return TradingModeDomain{value: selectableMode}, nil
		}
	}

	selectableSpellings := make([]string, 0, len(selectableTradingModes))
	for _, selectableMode := range selectableTradingModes {
		selectableSpellings = append(selectableSpellings, string(selectableMode))
	}

	return TradingModeDomain{}, BacktestValidationFailure(
		BacktestTradingModeField,
		fmt.Sprintf("交易模式只能是 %s 其中之一", strings.Join(selectableSpellings, "、")))
}

func (tradingModeDomain TradingModeDomain) Value() vo.TradingModeVo {
	return tradingModeDomain.value
}

// TargetFor is what this opinion asks the account to be holding once the candle is
// over, under this mode's rules.
//
// The two modes part company on exactly one row of the table — what a sell asks for —
// and that is the whole of the difference between being able to short and not.
// Everything else they answer identically, which is why a replay whose script never
// says sell produces the same report card either way.
func (tradingModeDomain TradingModeDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
	if signal.Value() == vo.SignalBuy {
		return vo.TargetPositionLong
	}

	if signal.Value() != vo.SignalSell {
		return vo.TargetPositionUnchanged
	}

	// Every mode is named here rather than one being the fall-through. A mode this
	// does not recognise — a zero value that never went through the constructor, or a
	// third one added to the selectable set and forgotten here — asks for nothing at
	// all, so the replay makes no trades.
	//
	// That is the loud failure of the two available. Falling through to a short would
	// produce a complete, entirely plausible long-short report card for a mode nobody
	// meant to replay, and nothing about it would look wrong.
	switch tradingModeDomain.value {
	case vo.TradingModeLongShort:
		return vo.TargetPositionShort
	case vo.TradingModeSpot:
		return vo.TargetPositionFlat
	}

	return vo.TargetPositionUnchanged
}
