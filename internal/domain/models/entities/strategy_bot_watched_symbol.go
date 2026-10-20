package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// StrategyBotWatchedSymbol is one symbol a bot applies its trading strategy to, with everything the bot remembers about that symbol alone.
type StrategyBotWatchedSymbol struct {
	ID            uint   `gorm:"primaryKey"`
	StrategyBotID uint   `gorm:"not null;uniqueIndex:idx_strategy_bot_watched_symbols_bot_symbol,priority:1"`
	Symbol        string `gorm:"size:64;not null;uniqueIndex:idx_strategy_bot_watched_symbols_bot_symbol,priority:2"`
	// LastSentSignal is cleared on start so the first conclusion after starting is always sent.
	LastSentSignal string `gorm:"size:16;not null;default:''"`
	// LastSentRoundDueAt names the round whose message carried LastSentSignal, so a message given up forgets the signal only if no later round has said it since.
	LastSentRoundDueAt *time.Time `gorm:"type:timestamptz"`
	// Conflicting is not a halt; it clears on the next non-conflicting round.
	Conflicting bool `gorm:"not null;default:false"`
	// HaltReason is empty unless the system stopped watching this symbol alone; the bot keeps running its other symbols.
	HaltReason string `gorm:"size:32;not null;default:''"`
	// AutoOrderPosition* is what the bot opened on this symbol with auto orders and has not closed: a direction (empty when flat), a quantity and the protective orders guarding it.
	AutoOrderPositionDirection  string          `gorm:"size:8;not null;default:''"`
	AutoOrderPositionQuantity   decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	AutoOrderStopLossClientID   string          `gorm:"size:64;not null;default:''"`
	AutoOrderTakeProfitClientID string          `gorm:"size:64;not null;default:''"`
	CreatedAt                   time.Time       `gorm:"type:timestamptz;not null"`
}

func (strategyBotWatchedSymbol StrategyBotWatchedSymbol) TableName() string {
	return "StrategyBotWatchedSymbols"
}

func (strategyBotWatchedSymbol StrategyBotWatchedSymbol) ToAutoOrderPositionVo() vo.AutoOrderPositionVo {
	return vo.AutoOrderPositionVo{
		Direction:          vo.TargetPositionVo(strategyBotWatchedSymbol.AutoOrderPositionDirection),
		Quantity:           strategyBotWatchedSymbol.AutoOrderPositionQuantity,
		StopLossClientID:   strategyBotWatchedSymbol.AutoOrderStopLossClientID,
		TakeProfitClientID: strategyBotWatchedSymbol.AutoOrderTakeProfitClientID,
	}
}

// ToDto shows the auto-order position only on contract bots, the only ones that open positions.
func (strategyBotWatchedSymbol StrategyBotWatchedSymbol) ToDto(marketDataKind string) dto.StrategyBotWatchedSymbolDto {
	watchedSymbolDto := dto.StrategyBotWatchedSymbolDto{
		Symbol:         strategyBotWatchedSymbol.Symbol,
		LastSentSignal: strategyBotWatchedSymbol.LastSentSignal,
		Conflicting:    strategyBotWatchedSymbol.Conflicting,
		HaltReason:     strategyBotWatchedSymbol.HaltReason,
	}
	if marketDataKind == string(vo.MarketDataKindContractKCandle) {
		watchedSymbolDto.AutoOrderPosition = &dto.AutoOrderPositionDto{
			Direction: strategyBotWatchedSymbol.AutoOrderPositionDirection,
			Quantity:  strategyBotWatchedSymbol.AutoOrderPositionQuantity,
		}
	}

	return watchedSymbolDto
}
