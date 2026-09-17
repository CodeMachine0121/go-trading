package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// The three kinds a round can end in, as the application layer names them on its
// way back in. They are strings on a DTO because a DTO is the only shape that layer
// may hold; which of the three this is, and what it does to a bot, is read here.
const (
	strategyBotRoundSkipped   = "skipped"
	strategyBotRoundHalted    = "halted"
	strategyBotRoundConcluded = "concluded"
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
	verdict     vo.StrategyBotVerdictVo
	sentSignal  vo.SignalVo
	conflicting bool
}

// NewStrategyBotRoundOutcomeDomainOf reads an outcome that came back from the
// application layer.
//
// Anything it does not recognise is read as a skipped round: that is the harmless
// reading — the bot moves on to its next round and nothing else about it changes —
// and the alternative would be inventing a halt nobody asked for.
func NewStrategyBotRoundOutcomeDomainOf(
	outcomeDto dto.StrategyBotRoundOutcomeDto,
) StrategyBotRoundOutcomeDomain {
	switch outcomeDto.Kind {
	case strategyBotRoundHalted:
		return NewStrategyBotRoundHaltedOutcome(
			vo.StrategyBotHaltReasonVo(outcomeDto.HaltReason))
	case strategyBotRoundConcluded:
		return NewStrategyBotRoundConcludedOutcome(
			vo.StrategyBotVerdictVo(outcomeDto.Verdict),
			vo.SignalVo(outcomeDto.SentSignal),
			outcomeDto.Conflicting)
	default:
		return NewStrategyBotRoundSkippedOutcome()
	}
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
	verdict vo.StrategyBotVerdictVo, sentSignal vo.SignalVo, conflicting bool,
) StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{
		verdict: verdict, sentSignal: sentSignal, conflicting: conflicting}
}

// RecordedResult is this round as its history remembers it: buy, sell, hold, or
// conflicted.
//
// Conflicted is told apart from hold, and the reason is the reader's next move. A
// holding bot is waiting for the market; a conflicted one is waiting for its owner,
// because both of its conditions held at once and it will stay silent until one of
// them is changed. Recording both as "hold" hid the only entry in a history that
// asks somebody to go and do something.
//
// Everything else that ends a round without a position — concluded nothing, skipped,
// halted the bot — is still one word. Why it was none of those is on the bot itself,
// in its halt reason, where it can actually be acted on.
//
// It reads the verdict and not the signal that was sent. A bot holding the same view
// for twelve rounds sent one message and thought "buy" twelve times, and the history
// is about what it thought.
func (strategyBotRoundOutcomeDomain StrategyBotRoundOutcomeDomain) RecordedResult() vo.StrategyBotRoundResultVo {
	switch strategyBotRoundOutcomeDomain.verdict {
	case vo.StrategyBotVerdictBuy:
		return vo.StrategyBotRoundResultBuy
	case vo.StrategyBotVerdictSell:
		return vo.StrategyBotRoundResultSell
	case vo.StrategyBotVerdictConflict:
		return vo.StrategyBotRoundResultConflict
	default:
		return vo.StrategyBotRoundResultHold
	}
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
