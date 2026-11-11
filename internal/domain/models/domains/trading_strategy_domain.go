package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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

	// A trading strategy only answers buy, sell or hold; what those mean is the bot's or the replay's to say, so a declared trading mode is refused rather than quietly dropped.
	if strings.TrimSpace(writeDto.DeclaredTradingMode) != "" {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: 交易策略不再記交易模式——交易方向改由機器人在自己身上選，回測時由那一次說",
			ErrTradingStrategyValidation)
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
		SignalSources:  tradingStrategyDomain.signalSources.ToEntities(),
		ConditionNodes: []entities.TradingStrategyConditionNode{
			tradingStrategyDomain.buyCondition.ToEntity(vo.TradingStrategyConditionSideBuy, 0),
			tradingStrategyDomain.sellCondition.ToEntity(vo.TradingStrategyConditionSideSell, 0),
		},
	}
}
