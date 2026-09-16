package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotRoundFailureDomain is the one place that decides what a failed round
// means: skip it and try again next time, or stop this bot and say why.
//
// It exists so that decision is written once. Spread across the call sites that
// produce failures, it would be a dozen `if` statements that have to agree with each
// other forever — and the day a new kind of failure appears, the one place it was
// forgotten is the place a bot quietly fails every five minutes while pretending to
// be alive.
//
// The dividing line is whether running it again could give a different answer. A
// closed market opens; missing candles arrive; Telegram comes back. A deleted
// strategy does not undelete itself, and a broken script does not fix itself.
//
// Anything unrecognised skips rather than halts. Halting is the destructive answer —
// it takes a bot its owner started and turns it off — so an unfamiliar failure gets
// the benign reading, and the worst case is a bot that retries something hopeless
// instead of one switched off by a database hiccup.
type StrategyBotRoundFailureDomain struct {
	haltReason vo.StrategyBotHaltReasonVo
}

// NewStrategyBotRoundFailureDomain reads a failure that happened while working out
// this round's signals.
func NewStrategyBotRoundFailureDomain(roundError error) StrategyBotRoundFailureDomain {
	switch {
	// Not being able to see a strategy is one fact with two histories — its owner
	// deleted it, or they withdrew it from the marketplace. They share one reason
	// here for the same cause they share one sentence everywhere else: telling them
	// apart would say whether somebody else's strategy still exists.
	case errors.Is(roundError, ErrStrategyNotFound),
		errors.Is(roundError, ErrStrategyNotPublished):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltStrategyUnavailable}

	case errors.Is(roundError, ErrIndicatorScriptFailed),
		errors.Is(roundError, ErrIndicatorParameterNotDeclared):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltScriptFailed}

	default:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltNone}
	}
}

// NewStrategyBotDeliveryFailureDomain reads a failure that happened while sending
// this round's message.
//
// This is what the four delivery failure reasons were separated for. A rejected
// token and an unknown chat need somebody to go and retype something, so the bot
// stops and says which; not being able to reach Telegram needs nothing but time, so
// the bot waits.
func NewStrategyBotDeliveryFailureDomain(
	failureReason vo.DeliveryFailureReasonVo,
) StrategyBotRoundFailureDomain {
	switch failureReason {
	case vo.DeliveryFailureCredentialRejected:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltCredentialRejected}
	case vo.DeliveryFailureDestinationNotFound:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltDestinationNotFound}
	default:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltNone}
	}
}

// HaltsTheBot says whether this failure is one that stops the bot.
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) HaltsTheBot() bool {
	return strategyBotRoundFailureDomain.haltReason != vo.StrategyBotHaltNone
}

// HaltReason is why, and is empty for a failure that only skips the round.
//
// Whether it halts is worked out from this rather than carried beside it, because
// two fields saying one thing are two fields that can disagree — and the pair that
// disagrees is "it halted, and here is no reason why".
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) HaltReason() vo.StrategyBotHaltReasonVo {
	return strategyBotRoundFailureDomain.haltReason
}
