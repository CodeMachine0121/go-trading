package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotWatchedSymbolDomain is one watched symbol's own state transitions; each method returns the whole record so a transition can't be half-applied.
type StrategyBotWatchedSymbolDomain struct {
	watchedSymbol entities.StrategyBotWatchedSymbol
}

func NewStrategyBotWatchedSymbolDomain(watchedSymbol entities.StrategyBotWatchedSymbol) StrategyBotWatchedSymbolDomain {
	return StrategyBotWatchedSymbolDomain{watchedSymbol: watchedSymbol}
}

func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) Symbol() string {
	return watchedSymbolDomain.watchedSymbol.Symbol
}

// IsHalted is a symbol the system stopped watching on its own; rounds pass it by until the bot is started again.
func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) IsHalted() bool {
	return vo.StrategyBotHaltReasonVo(watchedSymbolDomain.watchedSymbol.HaltReason) != vo.StrategyBotHaltNone
}

// RequireRemovable refuses dropping a symbol that still holds what auto orders opened, or has an auto order not yet done: once dropped, nothing would close or guard it.
func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) RequireRemovable(hasUnsettledAutoOrder bool) error {
	direction := vo.TargetPositionVo(watchedSymbolDomain.watchedSymbol.AutoOrderPositionDirection)
	if direction != "" && direction != vo.TargetPositionFlat {
		return fmt.Errorf("%w: %s 還有機器人持倉，要先讓它空手才能移除",
			ErrStrategyBotWatchedSymbolHoldsPosition, watchedSymbolDomain.watchedSymbol.Symbol)
	}

	if hasUnsettledAutoOrder {
		return fmt.Errorf("%w: %s 還有沒做完的自動下單，要等它做完才能移除",
			ErrStrategyBotWatchedSymbolAutoOrderOutstanding, watchedSymbolDomain.watchedSymbol.Symbol)
	}

	return nil
}

// Concluded moves the last sent signal, with the round that said it, only when the round queued a message.
func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) Concluded(
	dueAt time.Time, sentSignal vo.SignalVo, conflicting bool,
) entities.StrategyBotWatchedSymbol {
	concludedSymbol := watchedSymbolDomain.watchedSymbol
	concludedSymbol.Conflicting = conflicting

	if sentSignal != "" {
		dueAtUtc := dueAt.UTC()
		concludedSymbol.LastSentSignal = string(sentSignal)
		concludedSymbol.LastSentRoundDueAt = &dueAtUtc
	}

	return concludedSymbol
}

func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) Halted(
	haltReason vo.StrategyBotHaltReasonVo,
) entities.StrategyBotWatchedSymbol {
	haltedSymbol := watchedSymbolDomain.watchedSymbol
	haltedSymbol.HaltReason = string(haltReason)

	return haltedSymbol
}

// ClearedForStart forgets what was said and why the symbol halted, so the first conclusion after a start is always sent; what auto orders opened stays.
func (watchedSymbolDomain StrategyBotWatchedSymbolDomain) ClearedForStart() entities.StrategyBotWatchedSymbol {
	clearedSymbol := watchedSymbolDomain.watchedSymbol
	clearedSymbol.LastSentSignal = ""
	clearedSymbol.LastSentRoundDueAt = nil
	clearedSymbol.Conflicting = false
	clearedSymbol.HaltReason = string(vo.StrategyBotHaltNone)

	return clearedSymbol
}
