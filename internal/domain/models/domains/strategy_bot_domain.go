package domains

import (
	"fmt"
	"slices"
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

// StrategyBotWatchedSymbolLimit caps one bot's watched symbols, since every symbol multiplies the bot's per-round script runs.
const StrategyBotWatchedSymbolLimit = 10

// strategyBotEntrySlippageToleranceDefaultPercentage is a little wider than the slippage market entries actually paid, so ordinary moves still fill.
var (
	strategyBotEntrySlippageToleranceDefaultPercentage = decimal.RequireFromString("0.1")
	strategyBotEntrySlippageToleranceMaximumPercentage = decimal.NewFromInt(5)
)

// StrategyBotDomain is a validated bot being saved; it only checks that one trading strategy is named (ownership is the application's job) and knows nothing about run state (see StrategyBotRunStateDomain).
type StrategyBotDomain struct {
	id                     uint
	ownerID                uint
	name                   string
	symbols                []string
	marketDataKind         MarketDataKindDomain
	tradingStrategyID      uint
	triggerIntervalMinutes int
	positionPlan           PositionPlanDomain
	capitalAllocation      StrategyBotCapitalAllocationDomain
	// leverage is one or more for contract bots and zero for spot bots.
	leverage                         decimal.Decimal
	entrySlippageTolerancePercentage decimal.Decimal
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

	if len(writeDto.Symbols) == 0 {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須指定這台機器人要盯哪些交易標的", ErrStrategyBotValidation)
	}

	if len(writeDto.Symbols) > StrategyBotWatchedSymbolLimit {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 一台機器人最多盯 %d 個交易標的", ErrStrategyBotValidation, StrategyBotWatchedSymbolLimit)
	}

	// Validated by the shared symbol model because a mismatched symbol finds no candles and silently records every round as hold.
	symbols := make([]string, 0, len(writeDto.Symbols))
	seenSymbols := map[string]bool{}
	for _, requestedSymbol := range writeDto.Symbols {
		tradingSymbol, symbolError := NewTradingSymbolDomain(strings.TrimSpace(requestedSymbol))
		if symbolError != nil {
			return StrategyBotDomain{}, fmt.Errorf("%w: %w", ErrStrategyBotValidation, symbolError)
		}

		if seenSymbols[tradingSymbol.Value()] {
			return StrategyBotDomain{}, fmt.Errorf(
				"%w: 交易標的 %s 重複了", ErrStrategyBotValidation, tradingSymbol.Value())
		}
		seenSymbols[tradingSymbol.Value()] = true
		symbols = append(symbols, tradingSymbol.Value())
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

	capitalAllocation, allocationError := NewStrategyBotCapitalAllocationDomain(
		symbols, writeDto.CapitalAllocations, writeDto.RetainedCapitalAllocations)
	if allocationError != nil {
		return StrategyBotDomain{}, allocationError
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

	entrySlippageTolerancePercentage := strategyBotEntrySlippageToleranceDefaultPercentage
	if writeDto.AutoOrderEntrySlippageTolerancePercentage.Valid {
		entrySlippageTolerancePercentage = writeDto.AutoOrderEntrySlippageTolerancePercentage.Decimal
	}
	if entrySlippageTolerancePercentage.IsNegative() ||
		entrySlippageTolerancePercentage.GreaterThan(strategyBotEntrySlippageToleranceMaximumPercentage) {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 進場滑價容忍度必須介於 0 到 %s 之間（單位：%%）",
			ErrStrategyBotValidation, strategyBotEntrySlippageToleranceMaximumPercentage.String())
	}

	return StrategyBotDomain{
		id:                               writeDto.ID,
		ownerID:                          writeDto.OwnerID,
		name:                             name,
		symbols:                          symbols,
		marketDataKind:                   marketDataKind,
		tradingStrategyID:                writeDto.TradingStrategyID,
		triggerIntervalMinutes:           writeDto.TriggerIntervalMinutes,
		positionPlan:                     positionPlan,
		capitalAllocation:                capitalAllocation,
		leverage:                         leverage,
		entrySlippageTolerancePercentage: entrySlippageTolerancePercentage,
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

func (strategyBotDomain StrategyBotDomain) Symbols() []string {
	return strategyBotDomain.symbols
}

// WatchedSymbolsDroppedFrom are the stored bot's watched symbols this rewrite no longer lists.
func (strategyBotDomain StrategyBotDomain) WatchedSymbolsDroppedFrom(
	storedBot entities.StrategyBot,
) []StrategyBotWatchedSymbolDomain {
	droppedSymbols := []StrategyBotWatchedSymbolDomain{}
	for _, watchedSymbol := range storedBot.WatchedSymbols {
		if !slices.Contains(strategyBotDomain.symbols, watchedSymbol.Symbol) {
			droppedSymbols = append(droppedSymbols, NewStrategyBotWatchedSymbolDomain(watchedSymbol))
		}
	}

	return droppedSymbols
}

func (strategyBotDomain StrategyBotDomain) Leverage() decimal.Decimal {
	return strategyBotDomain.leverage
}

// ToEntity uses a new bot's default run state; rewrites share it since a bot can only be rewritten while stopped.
func (strategyBotDomain StrategyBotDomain) ToEntity() entities.StrategyBot {
	positionPlanSettings := strategyBotDomain.positionPlan.ToSettingsDto()

	watchedSymbols := make([]entities.StrategyBotWatchedSymbol, 0, len(strategyBotDomain.symbols))
	for _, symbol := range strategyBotDomain.symbols {
		watchedSymbols = append(watchedSymbols, entities.StrategyBotWatchedSymbol{
			StrategyBotID:        strategyBotDomain.id,
			Symbol:               symbol,
			AllocationPercentage: strategyBotDomain.capitalAllocation.PercentageOf(symbol),
		})
	}

	return entities.StrategyBot{
		ID:                               strategyBotDomain.id,
		OwnerID:                          strategyBotDomain.ownerID,
		Name:                             strategyBotDomain.name,
		MarketDataKind:                   string(strategyBotDomain.marketDataKind.Value()),
		TradingStrategyID:                strategyBotDomain.tradingStrategyID,
		TriggerIntervalMinutes:           strategyBotDomain.triggerIntervalMinutes,
		PositionPlanCapital:              positionPlanSettings.Capital,
		PositionPlanSizingMode:           positionPlanSettings.SizingMode,
		PositionPlanSizingValue:          positionPlanSettings.SizingValue,
		PositionPlanStopLossPercentage:   positionPlanSettings.StopLossPercentage,
		PositionPlanTakeProfitPercentage: positionPlanSettings.TakeProfitPercentage,
		PositionPlanLeverage:             strategyBotDomain.leverage,
		AutoOrderEntrySlippageTolerancePercentage: strategyBotDomain.entrySlippageTolerancePercentage,
		RunState:       string(vo.StrategyBotStopped),
		WatchedSymbols: watchedSymbols,
	}
}
