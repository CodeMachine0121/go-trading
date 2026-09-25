package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// tradingStrategyNameMaxLength matches the strategy script and bot name limits and applies
// after trimming.
const tradingStrategyNameMaxLength = 128

// TradingStrategyDomain settles sources before conditions because conditions may only name
// labels the sources declared.
type TradingStrategyDomain struct {
	id      uint
	ownerID uint
	name    string
	// marketDataKind also carries the trading mode for contract trading strategies.
	marketDataKind MarketDataKindDomain
	tradingMode    ContractTradingModeDomain
	signalSources  TradingStrategySignalSourcesDomain
	buyCondition   TradingStrategyConditionDomain
	sellCondition  TradingStrategyConditionDomain
}

func NewTradingStrategyDomain(writeDto dto.TradingStrategyWriteDto) (TradingStrategyDomain, error) {
	if writeDto.OwnerID == 0 {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: 交易策略必須屬於一位使用者", ErrTradingStrategyValidation)
	}

	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: 必須給交易策略取一個名稱", ErrTradingStrategyValidation)
	}

	if strings.ContainsRune(name, nulCharacter) {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: 交易策略名稱不得包含空字元（NUL）", ErrTradingStrategyValidation)
	}

	if len([]rune(name)) > tradingStrategyNameMaxLength {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: 交易策略名稱長度上限為 %d 個字",
			ErrTradingStrategyValidation, tradingStrategyNameMaxLength)
	}

	marketDataKind, kindError := NewMarketDataKindDomain(writeDto.MarketDataKind)
	if kindError != nil {
		return TradingStrategyDomain{}, fmt.Errorf("%w: %w", ErrTradingStrategyValidation, kindError)
	}

	// Spot trading strategies refuse any trading mode with the spot replay's wording; blank
	// means long and short for contracts.
	tradingMode := ContractTradingModeDomain{}
	if marketDataKind.Value() == vo.MarketDataKindKCandle {
		if _, spotOnlyRefusal := NewSpotOnlyReplayDomain(
			writeDto.TradingMode, decimal.Zero, decimal.Zero); spotOnlyRefusal != nil {
			return TradingStrategyDomain{}, fmt.Errorf(
				"%w: %s", ErrTradingStrategyValidation, spotOnlyRefusal)
		}
	} else {
		contractTradingMode, tradingModeError := NewContractTradingModeDomain(writeDto.TradingMode)
		if tradingModeError != nil {
			return TradingStrategyDomain{}, fmt.Errorf(
				"%w: %w", ErrTradingStrategyValidation, tradingModeError)
		}
		tradingMode = contractTradingMode
	}

	signalSources, sourcesError := NewTradingStrategySignalSourcesDomain(writeDto.SignalSources)
	if sourcesError != nil {
		return TradingStrategyDomain{}, sourcesError
	}

	if kindMismatch := signalSources.RequireMarketDataKind(marketDataKind); kindMismatch != nil {
		return TradingStrategyDomain{}, kindMismatch
	}

	buyCondition, buyError := NewTradingStrategyConditionDomain(
		writeDto.BuyCondition, signalSources.Labels())
	if buyError != nil {
		return TradingStrategyDomain{}, fmt.Errorf("%w（買入條件）", buyError)
	}

	sellCondition, sellError := NewTradingStrategyConditionDomain(
		writeDto.SellCondition, signalSources.Labels())
	if sellError != nil {
		return TradingStrategyDomain{}, fmt.Errorf("%w（賣出條件）", sellError)
	}

	return TradingStrategyDomain{
		id:             writeDto.ID,
		ownerID:        writeDto.OwnerID,
		name:           name,
		marketDataKind: marketDataKind,
		tradingMode:    tradingMode,
		signalSources:  signalSources,
		buyCondition:   buyCondition,
		sellCondition:  sellCondition,
	}, nil
}

func (tradingStrategyDomain TradingStrategyDomain) ToEntity() entities.TradingStrategy {
	return entities.TradingStrategy{
		ID:             tradingStrategyDomain.id,
		OwnerID:        tradingStrategyDomain.ownerID,
		Name:           tradingStrategyDomain.name,
		MarketDataKind: string(tradingStrategyDomain.marketDataKind.Value()),
		TradingMode:    string(tradingStrategyDomain.tradingMode.Value()),
		SignalSources:  tradingStrategyDomain.signalSources.ToEntities(),
		ConditionNodes: []entities.TradingStrategyConditionNode{
			tradingStrategyDomain.buyCondition.ToEntity(vo.TradingStrategyConditionSideBuy, 0),
			tradingStrategyDomain.sellCondition.ToEntity(vo.TradingStrategyConditionSideSell, 0),
		},
	}
}
