package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotRoundOutcomeDomain is one whole round across the bot's watched symbols, booked in one call so no exit path can forget to reschedule the bot.
type StrategyBotRoundOutcomeDomain struct {
	watchedSymbolOutcomes []StrategyBotWatchedSymbolRoundOutcomeDomain
}

func NewStrategyBotRoundOutcomeDomain(outcomeDto dto.StrategyBotRoundOutcomeDto) StrategyBotRoundOutcomeDomain {
	watchedSymbolOutcomes := make([]StrategyBotWatchedSymbolRoundOutcomeDomain, 0, len(outcomeDto.WatchedSymbolOutcomes))
	for _, watchedSymbolOutcomeDto := range outcomeDto.WatchedSymbolOutcomes {
		watchedSymbolOutcomes = append(watchedSymbolOutcomes,
			NewStrategyBotWatchedSymbolRoundOutcomeDomain(watchedSymbolOutcomeDto))
	}

	return StrategyBotRoundOutcomeDomain{watchedSymbolOutcomes: watchedSymbolOutcomes}
}

// ApplyTo writes every symbol's outcome onto the bot and returns the record to store with the reason this round halted the whole bot, empty when it did not.
// A failure every symbol would meet halts the bot at once; otherwise the bot halts only once no watched symbol is left, for the last reason a symbol halted this round, so a one-symbol bot halts exactly as before.
func (roundOutcome StrategyBotRoundOutcomeDomain) ApplyTo(
	runStateDomain StrategyBotRunStateDomain, dueAt time.Time, now time.Time,
) (entities.StrategyBot, vo.StrategyBotHaltReasonVo) {
	botHaltReason := vo.StrategyBotHaltNone
	lastWatchedSymbolHaltReason := vo.StrategyBotHaltNone

	for _, watchedSymbolOutcome := range roundOutcome.watchedSymbolOutcomes {
		runStateDomain = watchedSymbolOutcome.ApplyTo(runStateDomain, dueAt)

		if watchedSymbolOutcome.HaltsTheBot() {
			botHaltReason = watchedSymbolOutcome.HaltReason()
		}
		if watchedSymbolOutcome.HaltsTheWatchedSymbol() {
			lastWatchedSymbolHaltReason = watchedSymbolOutcome.HaltReason()
		}
	}

	if botHaltReason == vo.StrategyBotHaltNone &&
		lastWatchedSymbolHaltReason != vo.StrategyBotHaltNone && runStateDomain.AllWatchedSymbolsHalted() {
		botHaltReason = lastWatchedSymbolHaltReason
	}

	if botHaltReason != vo.StrategyBotHaltNone {
		return runStateDomain.Halt(botHaltReason, now), botHaltReason
	}

	return runStateDomain.Rescheduled(now), vo.StrategyBotHaltNone
}
