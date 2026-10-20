package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// StrategyBot is one standing bot following a trading strategy on the symbols it watches; its run state is persisted so it survives restarts.
type StrategyBot struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is never on the rewrite column list, so a bot cannot change hands.
	OwnerID uint   `gorm:"not null;index:idx_strategy_bots_owner;uniqueIndex:idx_strategy_bots_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategy_bots_owner_name"`
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
	// HaltReason is empty unless the system stopped this whole bot itself; a single symbol's halt lives on that watched symbol.
	HaltReason string `gorm:"size:32;not null;default:''"`
	// RoundClaimedBy and RoundClaimedUntil say which replica is running this bot's round and until when; a claim past its time is free to take, so a replica that dies mid-round never strands its bot.
	RoundClaimedBy    string     `gorm:"size:255;not null;default:''"`
	RoundClaimedUntil *time.Time `gorm:"type:timestamptz"`
	// AutoOrderEnabled is off for every existing bot; neither Save nor UpdateRunState names it, so only the switch routes change it.
	AutoOrderEnabled bool `gorm:"not null;default:false"`
	// AutoOrderEntrySlippageTolerancePercentage is how far, in percent of the reference price, an auto order's entry limit may sit from it; the default reaches existing rows when the column is added.
	AutoOrderEntrySlippageTolerancePercentage decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0.1"`
	CreatedAt                                 time.Time       `gorm:"type:timestamptz;not null"`
	UpdatedAt                                 time.Time       `gorm:"type:timestamptz;not null"`

	// WatchedSymbols are read in the order they were added.
	WatchedSymbols []StrategyBotWatchedSymbol `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// TradingStrategy has no constraint so it adds no foreign key; see TradingStrategyID.
	TradingStrategy TradingStrategy `gorm:"foreignKey:TradingStrategyID;references:ID;constraint:-"`
	// RunRecords is declared only so deleting a bot cascades to its history.
	RunRecords []StrategyBotRunRecord `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
	// PendingMessages is declared only so deleting a bot drops what it has not said yet.
	PendingMessages []PendingMessage `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
	// ContractAutoOrders is declared only so deleting a bot drops the orders it has not carried out.
	ContractAutoOrders []ContractAutoOrder `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
}

// WatchedSymbol finds one of the bot's watched symbols; false when the bot does not watch it.
func (strategyBot StrategyBot) WatchedSymbol(symbol string) (StrategyBotWatchedSymbol, bool) {
	for _, watchedSymbol := range strategyBot.WatchedSymbols {
		if watchedSymbol.Symbol == symbol {
			return watchedSymbol, true
		}
	}

	return StrategyBotWatchedSymbol{}, false
}

func (strategyBot StrategyBot) Symbols() []string {
	symbols := make([]string, 0, len(strategyBot.WatchedSymbols))
	for _, watchedSymbol := range strategyBot.WatchedSymbols {
		symbols = append(symbols, watchedSymbol.Symbol)
	}

	return symbols
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

	watchedSymbolDtos := make([]dto.StrategyBotWatchedSymbolDto, 0, len(strategyBot.WatchedSymbols))
	for _, watchedSymbol := range strategyBot.WatchedSymbols {
		watchedSymbolDtos = append(watchedSymbolDtos, watchedSymbol.ToDto(marketDataKind))
	}

	return dto.StrategyBotDto{
		ID:                     strategyBot.ID,
		OwnerID:                strategyBot.OwnerID,
		Name:                   strategyBot.Name,
		WatchedSymbols:         watchedSymbolDtos,
		MarketDataKind:         marketDataKind,
		TriggerIntervalMinutes: strategyBot.TriggerIntervalMinutes,
		PositionPlan:           strategyBot.PositionPlanSettingsDto(),
		NextRunAt:              strategyBot.NextRunAt.UTC(),
		TradingStrategyID:      strategyBot.TradingStrategyID,
		TradingStrategyName:    strategyBot.TradingStrategy.Name,
		RunState:               strategyBot.RunState,
		HaltReason:             strategyBot.HaltReason,
		AutoOrderEnabled:       strategyBot.AutoOrderEnabled,
		AutoOrderEntrySlippageTolerancePercentage: strategyBot.AutoOrderEntrySlippageTolerancePercentage,
		CreatedAt: strategyBot.CreatedAt.UTC(),
		UpdatedAt: strategyBot.UpdatedAt.UTC(),
	}
}
