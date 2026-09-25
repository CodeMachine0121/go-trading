package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// Round outcome kinds as carried on the application-layer DTO.
const (
	strategyBotRoundSkipped   = "skipped"
	strategyBotRoundHalted    = "halted"
	strategyBotRoundConcluded = "concluded"
)

// StrategyBotRoundOutcomeDomain is a round's skipped/halted/concluded outcome, booked in one call so no exit path can forget to reschedule the bot; separate constructors keep fields consistent.
type StrategyBotRoundOutcomeDomain struct {
	skipped     bool
	haltReason  vo.StrategyBotHaltReasonVo
	verdict     vo.StrategyBotVerdictVo
	sentSignal  vo.SignalVo
	conflicting bool
}

// NewStrategyBotRoundOutcomeDomainOf reads unrecognised kinds as skipped, the harmless reading.
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

func NewStrategyBotRoundSkippedOutcome() StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{skipped: true}
}

func NewStrategyBotRoundHaltedOutcome(
	haltReason vo.StrategyBotHaltReasonVo,
) StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{haltReason: haltReason}
}

// NewStrategyBotRoundConcludedOutcome takes the signal actually sent to Telegram, empty when nothing was sent.
func NewStrategyBotRoundConcludedOutcome(
	verdict vo.StrategyBotVerdictVo, sentSignal vo.SignalVo, conflicting bool,
) StrategyBotRoundOutcomeDomain {
	return StrategyBotRoundOutcomeDomain{
		verdict: verdict, sentSignal: sentSignal, conflicting: conflicting}
}

// RecordedResult comes from the verdict, not the sent signal, and keeps conflict distinct from hold because a conflicted bot needs its owner to change a condition.
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

// ApplyTo writes the outcome onto the bot's run state and returns the record to store.
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
