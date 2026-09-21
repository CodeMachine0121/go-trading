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
	vo.TradingModeShortOnly,
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
	return tradingModeDomain.value == vo.TradingModeLongShort ||
		tradingModeDomain.value == vo.TradingModeShortOnly
}

// CanGoLong is whether these rules may hold a position that gains when the price
// rises.
//
// It reads like a question nobody would need to ask, and until short-only existed
// nobody did: every mode could go long, so the answer was a constant and the question
// was never written down. What was asked in its place was CanGoShort, whose false
// answer stood in for "long only" — and that stand-in is what short-only breaks. It
// shorts, so CanGoShort says yes; it cannot go long, and nothing said so.
//
// Asking it out loud is what stops a mode that cannot go long from being told 做多,
// in a message that would otherwise look entirely ordinary.
func (tradingModeDomain TradingModeDomain) CanGoLong() bool {
	return tradingModeDomain.value == vo.TradingModeLongShort ||
		tradingModeDomain.value == vo.TradingModeSpot ||
		tradingModeDomain.value == vo.TradingModeLeveragedLong
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
// It is its own question rather than a reading of either direction, and all three now
// give different answers: leveraged-long borrows without ever facing the other way,
// and short-only faces the other way without ever facing this one.
//
// Only one combination is impossible, and it is the one it always was: shorting
// without borrowing. Selling what you do not have means borrowing it first, so every
// mode that can short can also borrow.
//
// A zero value answers no, by the same rule the rest of this model follows: a mode
// this does not recognise is the last place to start lending.
func (tradingModeDomain TradingModeDomain) CanUseLeverage() bool {
	return tradingModeDomain.value == vo.TradingModeLongShort ||
		tradingModeDomain.value == vo.TradingModeLeveragedLong ||
		tradingModeDomain.value == vo.TradingModeShortOnly
}

// ActNeedsTheModeNamed is whether a reader told only the verb still has something
// left to find out about what they are being asked to do.
//
// Cash for goods is the one case where they do not: 買入 there means handing money
// over for a thing, full stop, and a line naming the mode would be a sentence about
// the system rather than about the market. Every other set of rules carries something
// the verb cannot say — that the act may be opening a position rather than closing
// one, or that the position is held on somebody else's money and can be taken away at
// a price. A reader who executes that in a cash frame of mind has been misled by a
// message that was technically correct.
//
// It is one question here rather than two predicates combined where the message is
// written, because combining them is a decision, and a decision at the call site is
// one the next mode can land on the wrong side of without anything saying so.
//
// **The shorting half is currently subsumed by the borrowing half**, and no test can
// tell the two apart: every mode that can short can also borrow, because selling what
// you do not have means borrowing it first. It is written out anyway because the two
// are separate reasons — one is about which way the act faces, the other about whose
// money it is held on — and reading only the borrowing half would leave the next
// person thinking a mode names itself because of leverage alone.
func (tradingModeDomain TradingModeDomain) ActNeedsTheModeNamed() bool {
	return tradingModeDomain.CanGoShort() || tradingModeDomain.CanUseLeverage()
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
	case vo.TradingModeShortOnly:
		return "只做空"
	}

	return string(tradingModeDomain.value)
}

// TargetFor is what this opinion asks the account to be holding once the candle is
// over, under this mode's rules.
//
// Each signal asks for the position it names if these rules can face that way, and
// for cash if they cannot. That is the whole table — it is not written out mode by
// mode because writing it out is how a mode ends up in the wrong row: the version
// before short-only answered every buy with a long, which was true of all three modes
// then and silently wrong for the first one that could not go long.
func (tradingModeDomain TradingModeDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
	// Facing neither way is a mode this does not recognise — a zero value that never
	// went through the constructor, or a new one added to the selectable set and
	// forgotten in the two predicates. It asks for nothing at all, so the replay makes
	// no trades.
	//
	// That is the loud failure of the ones available, and the reason it is checked
	// before the signal rather than falling out of it: every signal would otherwise ask
	// such a mode for cash, quietly closing positions nobody meant to close, and the
	// report card would look entirely plausible.
	if !tradingModeDomain.CanGoLong() && !tradingModeDomain.CanGoShort() {
		return vo.TargetPositionUnchanged
	}

	switch signal.Value() {
	case vo.SignalBuy:
		if tradingModeDomain.CanGoLong() {
			return vo.TargetPositionLong
		}

		return vo.TargetPositionFlat
	case vo.SignalSell:
		if tradingModeDomain.CanGoShort() {
			return vo.TargetPositionShort
		}

		return vo.TargetPositionFlat
	}

	return vo.TargetPositionUnchanged
}

// HeadlineVerbFor is what this round asks its reader to go and do.
//
// A conclusion is read as an instruction, and the signal's own word is not always one
// the reader can carry out. Told 賣出 by rules that cannot short, they ask what they
// are meant to be selling — they are flat, which is most of the time, because a sell
// reaches them on the strength of the signal alone and the signal has never known
// what they hold. Told 買入 by rules that cannot go long, they ask exactly the same
// question. 出場 is the one wording both of those readers can act on: there is a
// position, or there is not.
//
// It is one method rather than a branch at the call site because the table is four
// modes by three signals, and twelve cells written where they are read is twelve
// chances to write one of them backwards — a mistake nothing reports, because 做多 is
// a perfectly ordinary word to find in a message.
//
// Only the conclusion speaks of acts. The source lines below it keep quoting the
// scripts in 買入／賣出／持有, which is how a reader works back from the conclusion.
func (tradingModeDomain TradingModeDomain) HeadlineVerbFor(signal SignalDomain) string {
	// A mode this does not recognise keeps the signal's own vocabulary. A message is
	// the last place to invent a direction for somebody.
	if !tradingModeDomain.CanGoLong() && !tradingModeDomain.CanGoShort() {
		return signal.InWords()
	}

	switch signal.Value() {
	case vo.SignalBuy:
		if !tradingModeDomain.CanGoLong() {
			return "出場"
		}

		// Rules that face both ways are opening something either way, so the act is
		// named after the direction. Rules that only go long are handing over cash for
		// a thing, and 買入 is exactly that with no second reading.
		if tradingModeDomain.CanGoShort() {
			return "做多"
		}

		return "買入"
	case vo.SignalSell:
		if tradingModeDomain.CanGoShort() {
			return "做空"
		}

		return "出場"
	}

	return signal.InWords()
}
