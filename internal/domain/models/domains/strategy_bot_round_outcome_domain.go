package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotRoundOutcomeDomain is what one round came to, in the only three forms
// a round can end in: it was skipped, it stopped the bot, or it reached a conclusion.
//
// It exists so that booking a round in is one call rather than three. A round has
// several ways out, and with one recording method per way, every one of them has to
// remember to call its own — while the one that forgets leaves a bot due forever,
// running flat out against the database and Telegram, looking from the outside
// exactly like a bot that is working.
//
// The three constructors are what keep its fields from contradicting each other. A
// single shape with a flag per outcome could say "skipped, and here is the signal it
// sent", and nothing would be able to refuse it.
type StrategyBotRoundOutcomeDomain struct {
	skipped     bool
	haltReason  vo.StrategyBotHaltReasonVo
	sentSignal  vo.SignalVo
	conflicting bool
}

// NewStrategyBotRoundSkippedOutcome is a round that could not reach a conclusion for
// a reason that may well have gone by the next one. Nothing about the bot changes
// but when it is next due.
func NewStrategyBotRoundSkippedOutcome() StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{skipped: true}
}

// NewStrategyBotRoundHaltedOutcome is a round that hit something no amount of
// waiting fixes, and stops the bot with the reason somebody has to act on.
func NewStrategyBotRoundHaltedOutcome(
	haltReason vo.StrategyBotHaltReasonVo,
) StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{haltReason: haltReason}
}

// NewStrategyBotRoundConcludedOutcome is a round that ran through. The signal is
// what actually reached Telegram, and is empty when nothing was sent — a conclusion
// nobody received has not been said.
func NewStrategyBotRoundConcludedOutcome(
	sentSignal vo.SignalVo, conflicting bool,
) StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{sentSignal: sentSignal, conflicting: conflicting}
}

// ApplyTo is this outcome written onto the bot it happened to, handing back the
// record as it should now be stored.
//
// Which of the three shapes the outcome is, is read here and nowhere else, so no
// caller ever has to know there are three.
func (strategyBotRoundOutcomeDomain StrategyBotRoundOutcomeDomain) ApplyTo(
	runStateDomain StrategyBotRunStateDomain, now time.Time,
) entities.StrategyBot {
	if strategyBotRoundOutcomeDomain.haltReason != vo.StrategyBotHaltNone {
		return runStateDomain.Halt(strategyBotRoundOutcomeDomain.haltReason)
	}

	if strategyBotRoundOutcomeDomain.skipped {
		return runStateDomain.RoundSkipped(now)
	}

	return runStateDomain.RoundFinished(
		now, strategyBotRoundOutcomeDomain.sentSignal, strategyBotRoundOutcomeDomain.conflicting)
}
