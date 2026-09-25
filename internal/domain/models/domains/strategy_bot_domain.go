package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// strategyBotNameMaxLength matches the strategy script name limit.
const strategyBotNameMaxLength = 128

// strategyBotTriggerIntervalMinimumMinutes is one minute because that is the finest stored candle.
const strategyBotTriggerIntervalMinimumMinutes = 1

const strategyBotTriggerIntervalMaximumMinutes = 1440

// StrategyBotDomain is a validated bot being saved; it only checks that one trading strategy is named (ownership is the application's job) and knows nothing about run state (see StrategyBotRunStateDomain).
type StrategyBotDomain struct {
	id                     uint
	ownerID                uint
	name                   string
	symbol                 string
	marketDataKind         MarketDataKindDomain
	tradingStrategyID      uint
	triggerIntervalMinutes int
	positionPlan           PositionPlanDomain
	// leverage is one or more for contract bots and zero for spot bots.
	leverage decimal.Decimal
}

func NewStrategyBotDomain(writeDto dto.StrategyBotWriteDto) (StrategyBotDomain, error) {
	if writeDto.OwnerID == 0 {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 策略機器人必須屬於一位使用者", ErrStrategyBotValidation)
	}

	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須給策略機器人取一個名稱", ErrStrategyBotValidation)
	}

	if strings.ContainsRune(name, nulCharacter) {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 機器人名稱不得包含空字元（NUL）", ErrStrategyBotValidation)
	}

	if len([]rune(name)) > strategyBotNameMaxLength {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 機器人名稱長度上限為 %d 個字", ErrStrategyBotValidation, strategyBotNameMaxLength)
	}

	// Validated by the shared symbol model because a mismatched symbol finds no candles and silently records every round as hold.
	tradingSymbol, symbolError := NewTradingSymbolDomain(strings.TrimSpace(writeDto.Symbol))
	if symbolError != nil {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須指定這台機器人要盯哪一個交易標的", ErrStrategyBotValidation)
	}

	marketDataKind, kindError := NewMarketDataKindDomain(writeDto.MarketDataKind)
	if kindError != nil {
		return StrategyBotDomain{}, fmt.Errorf("%w: %w", ErrStrategyBotValidation, kindError)
	}

	if writeDto.TriggerIntervalMinutes < strategyBotTriggerIntervalMinimumMinutes {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 觸發間隔必須大於零，最短 %d 分鐘",
			ErrStrategyBotValidation, strategyBotTriggerIntervalMinimumMinutes)
	}

	if writeDto.TriggerIntervalMinutes > strategyBotTriggerIntervalMaximumMinutes {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 觸發間隔上限是 %d 分鐘",
			ErrStrategyBotValidation, strategyBotTriggerIntervalMaximumMinutes)
	}

	// Shares PositionPlanDomain with replays so bots and replays size positions identically.
	positionPlan, positionPlanError := NewPositionPlanDomain(writeDto.PositionPlan)
	if positionPlanError != nil {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: %s", ErrStrategyBotValidation, positionPlanError)
	}

	// Ownership of the named strategy is checked by the application.
	if writeDto.TradingStrategyID == 0 {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須指名這台機器人要用哪一份交易策略", ErrStrategyBotValidation)
	}

	leverage, leverageError := marketDataKind.LeverageForStrategyBot(writeDto.DeclaredLeverage)
	if leverageError != nil {
		return StrategyBotDomain{}, leverageError
	}

	return StrategyBotDomain{
		id:                     writeDto.ID,
		ownerID:                writeDto.OwnerID,
		name:                   name,
		symbol:                 tradingSymbol.Value(),
		marketDataKind:         marketDataKind,
		tradingStrategyID:      writeDto.TradingStrategyID,
		triggerIntervalMinutes: writeDto.TriggerIntervalMinutes,
		positionPlan:           positionPlan,
		leverage:               leverage,
	}, nil
}

// RequireFollowing refuses a trading strategy written for a different market data kind than this bot's.
func (strategyBotDomain StrategyBotDomain) RequireFollowing(tradingStrategyMarketDataKind string) error {
	tradingStrategyKind, kindError := NewMarketDataKindDomain(tradingStrategyMarketDataKind)
	if kindError != nil {
		return fmt.Errorf("%w: %w", ErrStrategyBotValidation, kindError)
	}

	return tradingStrategyKind.RequireFollowableByStrategyBotOf(strategyBotDomain.marketDataKind)
}

// WatchesContracts reports whether this bot trades perpetual contract bars.
func (strategyBotDomain StrategyBotDomain) WatchesContracts() bool {
	return strategyBotDomain.marketDataKind.IsContract()
}

func (strategyBotDomain StrategyBotDomain) Symbol() string {
	return strategyBotDomain.symbol
}

func (strategyBotDomain StrategyBotDomain) Leverage() decimal.Decimal {
	return strategyBotDomain.leverage
}

// ToEntity uses a new bot's default run state; rewrites share it since a bot can only be rewritten while stopped.
func (strategyBotDomain StrategyBotDomain) ToEntity() entities.StrategyBot {
	positionPlanSettings := strategyBotDomain.positionPlan.ToSettingsDto()

	return entities.StrategyBot{
		ID:                               strategyBotDomain.id,
		OwnerID:                          strategyBotDomain.ownerID,
		Name:                             strategyBotDomain.name,
		Symbol:                           strategyBotDomain.symbol,
		MarketDataKind:                   string(strategyBotDomain.marketDataKind.Value()),
		TradingStrategyID:                strategyBotDomain.tradingStrategyID,
		TriggerIntervalMinutes:           strategyBotDomain.triggerIntervalMinutes,
		PositionPlanCapital:              positionPlanSettings.Capital,
		PositionPlanSizingMode:           positionPlanSettings.SizingMode,
		PositionPlanSizingValue:          positionPlanSettings.SizingValue,
		PositionPlanStopLossPercentage:   positionPlanSettings.StopLossPercentage,
		PositionPlanTakeProfitPercentage: positionPlanSettings.TakeProfitPercentage,
		PositionPlanLeverage:             strategyBotDomain.leverage,
		RunState:                         string(vo.StrategyBotStopped),
	}
}
