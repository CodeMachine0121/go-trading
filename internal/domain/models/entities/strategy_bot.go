package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// StrategyBot is one standing bot following a trading strategy on one market; its run state is persisted so it survives restarts.
type StrategyBot struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is never on the rewrite column list, so a bot cannot change hands.
	OwnerID uint   `gorm:"not null;index:idx_strategy_bots_owner;uniqueIndex:idx_strategy_bots_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategy_bots_owner_name"`
	Symbol  string `gorm:"size:64;not null"`
	// MarketDataKind is fixed at creation; blank rows predate the choice and are K candle bots.
	MarketDataKind string `gorm:"size:32;not null;default:kCandle"`
	// TradingStrategyID has no foreign key because existing rows carried zero during the migration; deleting a followed strategy is refused a layer up instead.
	TradingStrategyID uint `gorm:"not null;default:0;index:idx_strategy_bots_trading_strategy"`
	// TriggerIntervalMinutes is in minutes because the finest candle is one minute.
	TriggerIntervalMinutes int `gorm:"not null"`
	// PositionPlanCapital of zero means the bot has no position plan; all plan fields default to zero so older bots read as planless.
	PositionPlanCapital     decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	PositionPlanSizingMode  string          `gorm:"size:16;not null;default:''"`
	PositionPlanSizingValue decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	// Either exit percentage may be left out on its own.
	PositionPlanStopLossPercentage   decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	PositionPlanTakeProfitPercentage decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	// PositionPlanLeverage is always zero for spot bots; it uses a new column so stale spot-era values in the retired position_plan_leverage column are never read.
	PositionPlanLeverage decimal.Decimal `gorm:"column:position_plan_contract_leverage;type:numeric(38,18);not null;default:0"`
	// RunState is indexed with NextRunAt for the scheduler's "running and due" scan.
	RunState string `gorm:"size:16;not null;index:idx_strategy_bots_run_state_next_run_at,priority:1"`
	// NextRunAt is measured from the last actual round, so missed rounds are never made up.
	NextRunAt time.Time `gorm:"type:timestamptz;index:idx_strategy_bots_run_state_next_run_at,priority:2"`
	// LastSentSignal is cleared on start so the first conclusion after starting is always sent.
	LastSentSignal string `gorm:"size:16;not null;default:''"`
	// HaltReason is empty unless the system stopped this bot itself.
	HaltReason string `gorm:"size:32;not null;default:''"`
	// Conflicting is not a halt; it clears on the next non-conflicting round.
	Conflicting bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// TradingStrategy has no constraint so it adds no foreign key; see TradingStrategyID.
	TradingStrategy TradingStrategy `gorm:"foreignKey:TradingStrategyID;references:ID;constraint:-"`
	// RunRecords is declared only so deleting a bot cascades to its history.
	RunRecords []StrategyBotRunRecord `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
}

func (strategyBot StrategyBot) PositionPlanSettingsDto() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              strategyBot.PositionPlanCapital,
		SizingMode:           strategyBot.PositionPlanSizingMode,
		SizingValue:          strategyBot.PositionPlanSizingValue,
		StopLossPercentage:   strategyBot.PositionPlanStopLossPercentage,
		TakeProfitPercentage: strategyBot.PositionPlanTakeProfitPercentage,
		Leverage:             strategyBot.PositionPlanLeverage,
	}
}

func (strategyBot StrategyBot) TableName() string {
	return "StrategyBots"
}

// ToDto reads the trading strategy name through the association so a rename is always reflected.
func (strategyBot StrategyBot) ToDto() dto.StrategyBotDto {
	marketDataKind := strategyBot.MarketDataKind
	if marketDataKind == "" {
		marketDataKind = string(vo.MarketDataKindKCandle)
	}

	return dto.StrategyBotDto{
		ID:                     strategyBot.ID,
		OwnerID:                strategyBot.OwnerID,
		Name:                   strategyBot.Name,
		Symbol:                 strategyBot.Symbol,
		MarketDataKind:         marketDataKind,
		TriggerIntervalMinutes: strategyBot.TriggerIntervalMinutes,
		PositionPlan:           strategyBot.PositionPlanSettingsDto(),
		NextRunAt:              strategyBot.NextRunAt.UTC(),
		TradingStrategyID:      strategyBot.TradingStrategyID,
		TradingStrategyName:    strategyBot.TradingStrategy.Name,
		RunState:               strategyBot.RunState,
		LastSentSignal:         strategyBot.LastSentSignal,
		HaltReason:             strategyBot.HaltReason,
		Conflicting:            strategyBot.Conflicting,
		CreatedAt:              strategyBot.CreatedAt.UTC(),
		UpdatedAt:              strategyBot.UpdatedAt.UTC(),
	}
}
