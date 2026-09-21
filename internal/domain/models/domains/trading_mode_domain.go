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
	vo.TradingModeLeveragedLong,
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

	// The refusal carries the sentence and nothing else. Two worlds ask this
	// question now — a replay and a set of rules being saved — and each has its own
	// sentinel to wrap it in so that its own controller keeps mapping it. Writing
	// the sentence here once is what keeps the two from drifting into two different
	// lists of what a trading mode may be.
	return TradingModeDomain{}, fmt.Errorf(
		"交易模式只能是 %s 其中之一", strings.Join(selectableSpellings, "、"))
}

func (tradingModeDomain TradingModeDomain) Value() vo.TradingModeVo {
	return tradingModeDomain.value
}

// CanGoShort is one of the two questions a mode answers: whether these rules may hold
// a position that gains when the price falls.
//
// It is named after the mode rather than after what any caller does with the answer.
// A message asks it to decide whether its headline says "sell" or "open a short", and
// a message's wording is the kind of thing that gains a second form next month —
// whereas "can these rules short" is settled for as long as the mode exists.
//
// A zero value — one that never went through the constructor — answers no, by the same
// rule TargetFor answers "unchanged": a mode this does not recognise is the last place
// to guess which way somebody should trade.
func (tradingModeDomain TradingModeDomain) CanGoShort() bool {
	return tradingModeDomain.value == vo.TradingModeLongShort
}

// CanUseLeverage is whether these rules may put on a position worth more than the
// money behind it.
//
// It is named after the mode rather than after what any caller does with the answer,
// for the reason CanGoShort is: a replay asks it to decide whether to refuse a
// multiplier, and what a refusal says is the kind of thing that gains a second form
// next month — whereas "can these rules borrow" is settled for as long as the mode
// exists.
//
// It is a second question rather than a reading of CanGoShort, and the two now give
// different answers: leveraged-long borrows without ever facing the other way, which
// is what a venue lending against long-only positions looks like.
//
// Those two questions have four combinations and only three modes, because the fourth
// cannot exist: selling what you do not have means borrowing it first, so nothing can
// short without also being able to borrow.
//
// A zero value answers no, by the same rule the rest of this model follows: a mode
// this does not recognise is the last place to start lending.
func (tradingModeDomain TradingModeDomain) CanUseLeverage() bool {
	return tradingModeDomain.value == vo.TradingModeLongShort ||
		tradingModeDomain.value == vo.TradingModeLeveragedLong
}

// BorrowingRefusal is why these rules may not hold a position worth more than the
// money behind them — or nil when they may.
//
// It is the sentence rather than the answer because two worlds now ask this: a replay
// being handed a multiplier, and a bot being saved with one in its position plan. Both
// have to refuse in the same words, and the only way two refusals stay identical is
// for there to be one of them.
//
// It lives here for the reason the question does: refusing needs both whether the mode
// may borrow and what the mode is called, and this is the only model that has either.
func (tradingModeDomain TradingModeDomain) BorrowingRefusal() error {
	if tradingModeDomain.CanUseLeverage() {
		return nil
	}

	return fmt.Errorf(
		"%s交易模式開不了槓桿——現貨是拿現金換東西，沒有人借錢給你", tradingModeDomain.InWords())
}

// InWords is this mode as a person reads it.
//
// Every mode is named rather than one being the fall-through, for the reason TargetFor
// names them all: a third mode added to the selectable set and forgotten here comes out
// as its own stored spelling, which is odd to read but true — not as one of the modes
// this does recognise.
func (tradingModeDomain TradingModeDomain) InWords() string {
	switch tradingModeDomain.value {
	case vo.TradingModeLongShort:
		return "多空反手"
	case vo.TradingModeSpot:
		return "現貨"
	case vo.TradingModeLeveragedLong:
		return "槓桿做多"
	}

	return string(tradingModeDomain.value)
}

// TargetFor is what this opinion asks the account to be holding once the candle is
// over, under this mode's rules.
//
// The modes part company on exactly one row of the table — what a sell asks for —
// and that is the whole of the difference between being able to short and not.
// Everything else they answer identically, which is why a replay whose script never
// says sell produces the same report card whichever mode ran it, and why spot and
// leveraged-long differ here not at all: they differ over borrowing, not direction.
func (tradingModeDomain TradingModeDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
	if signal.Value() == vo.SignalBuy {
		return vo.TargetPositionLong
	}

	if signal.Value() != vo.SignalSell {
		return vo.TargetPositionUnchanged
	}

	// Every mode is named here rather than one being the fall-through. A mode this
	// does not recognise — a zero value that never went through the constructor, or a
	// new one added to the selectable set and forgotten here — asks for nothing at
	// all, so the replay makes no trades.
	//
	// That is the loud failure of the ones available. Falling through to a short would
	// produce a complete, entirely plausible long-short report card for a mode nobody
	// meant to replay, and nothing about it would look wrong.
	switch tradingModeDomain.value {
	case vo.TradingModeLongShort:
		return vo.TargetPositionShort
	case vo.TradingModeSpot, vo.TradingModeLeveragedLong:
		return vo.TargetPositionFlat
	}

	return vo.TargetPositionUnchanged
}
