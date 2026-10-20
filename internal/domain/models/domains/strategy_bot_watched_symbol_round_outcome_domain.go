package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// Watched symbol round outcome kinds as carried on the application-layer DTO.
const (
	strategyBotRoundSkipped             = "skipped"
	strategyBotRoundHalted              = "halted"
	strategyBotRoundWatchedSymbolHalted = "watchedSymbolHalted"
	strategyBotRoundConcluded           = "concluded"
)

// StrategyBotWatchedSymbolRoundOutcomeDomain is what one round came to on one watched symbol: skipped, halting the whole bot, halting this symbol alone, or concluded.
type StrategyBotWatchedSymbolRoundOutcomeDomain struct {
	symbol      string
	kind        string
	haltReason  vo.StrategyBotHaltReasonVo
	verdict     vo.StrategyBotVerdictVo
	sentSignal  vo.SignalVo
	conflicting bool
}

// NewStrategyBotWatchedSymbolRoundOutcomeDomain reads unrecognised kinds, and halts without a reason, as skipped, the harmless reading.
func NewStrategyBotWatchedSymbolRoundOutcomeDomain(
	outcomeDto dto.StrategyBotWatchedSymbolRoundOutcomeDto,
) StrategyBotWatchedSymbolRoundOutcomeDomain {
	outcome := StrategyBotWatchedSymbolRoundOutcomeDomain{symbol: outcomeDto.Symbol, kind: strategyBotRoundSkipped}

	switch outcomeDto.Kind {
	case strategyBotRoundHalted, strategyBotRoundWatchedSymbolHalted:
		if vo.StrategyBotHaltReasonVo(outcomeDto.HaltReason) != vo.StrategyBotHaltNone {
			outcome.kind = outcomeDto.Kind
			outcome.haltReason = vo.StrategyBotHaltReasonVo(outcomeDto.HaltReason)
		}
	case strategyBotRoundConcluded:
		outcome.kind = strategyBotRoundConcluded
		outcome.verdict = vo.StrategyBotVerdictVo(outcomeDto.Verdict)
		outcome.sentSignal = vo.SignalVo(outcomeDto.SentSignal)
		outcome.conflicting = outcomeDto.Conflicting
	}

	return outcome
}

func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) Symbol() string {
	return outcome.symbol
}

// HaltsTheBot is a failure every symbol would meet alike, so the whole bot stops.
func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) HaltsTheBot() bool {
	return outcome.kind == strategyBotRoundHalted
}

// HaltsTheWatchedSymbol is a failure of this symbol alone; the bot keeps running its other symbols.
func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) HaltsTheWatchedSymbol() bool {
	return outcome.kind == strategyBotRoundWatchedSymbolHalted
}

func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) HaltReason() vo.StrategyBotHaltReasonVo {
	return outcome.haltReason
}

// RecordedResult comes from the verdict, not the sent signal, and keeps conflict distinct from hold because a conflicted symbol needs its owner to change a condition.
func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) RecordedResult() vo.StrategyBotRoundResultVo {
	switch outcome.verdict {
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

// ApplyTo writes a concluded or symbol-halting outcome onto the bot's watched symbol; a skip, a bot-wide halt or a symbol the bot no longer watches changes nothing here.
func (outcome StrategyBotWatchedSymbolRoundOutcomeDomain) ApplyTo(
	runStateDomain StrategyBotRunStateDomain, dueAt time.Time,
) StrategyBotRunStateDomain {
	watchedSymbol, isWatched := runStateDomain.WatchedSymbol(outcome.symbol)
	if !isWatched {
		return runStateDomain
	}

	switch outcome.kind {
	case strategyBotRoundWatchedSymbolHalted:
		return runStateDomain.WithWatchedSymbol(watchedSymbol.Halted(outcome.haltReason))
	case strategyBotRoundConcluded:
		return runStateDomain.WithWatchedSymbol(
			watchedSymbol.Concluded(dueAt, outcome.sentSignal, outcome.conflicting))
	default:
		return runStateDomain
	}
}
