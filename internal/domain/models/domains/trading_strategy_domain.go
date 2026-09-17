package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// tradingStrategyNameMaxLength is how long a trading strategy's name may be, counted
// after the blanks around it are dropped. It matches a strategy script's limit and a
// bot's rather than picking a third number, because there is no reason the three
// would ever want to differ and three numbers are three things to keep in step.
const tradingStrategyNameMaxLength = 128

// TradingStrategyDomain holds one trading strategy as it is being saved and
// guarantees its own invariants. An instance only exists when every rule passed, so
// there is no half-valid trading strategy.
//
// It delegates the two halves that have rules of their own — the sources and the
// conditions — rather than restating them. That is also what makes the ordering here
// matter: the sources are settled first because the conditions may only name labels
// the sources declared, and the labels are not known until the sources are.
//
// It knows nothing about any bot. Which market to watch, how often, and whether
// anything is running are questions about a machine following these rules, not about
// the rules — which is the whole reason the two are separate things.
type TradingStrategyDomain struct {
	id            uint
	ownerID       uint
	name          string
	tradingMode   TradingModeDomain
	signalSources TradingStrategySignalSourcesDomain
	buyCondition  TradingStrategyConditionDomain
	sellCondition TradingStrategyConditionDomain
}

// NewTradingStrategyDomain validates it against every rule that applies. The rules
// are identical whether it is being created or rewritten, because both arrive here
// as the same shape.
func NewTradingStrategyDomain(writeDto dto.TradingStrategyWriteDto) (TradingStrategyDomain, error) {
	// One with nobody behind it is refused here rather than at the store, because
	// "every trading strategy has an owner" is a rule about trading strategies, not
	// a constraint that happens to exist on a column.
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

	// Settled before the sources and the conditions, beside the name: the mode and the
	// name are what this set of rules *is*, while the sources and the two trees are
	// what it is made of. It reads its own declaration — blank means always in the
	// market and anything unrecognised is refused — in the same words a replay reads,
	// because there is only one model that knows what a trading mode may be.
	tradingMode, tradingModeError := NewTradingModeDomain(writeDto.TradingMode)
	if tradingModeError != nil {
		return TradingStrategyDomain{}, fmt.Errorf(
			"%w: %s", ErrTradingStrategyValidation, tradingModeError)
	}

	signalSources, sourcesError := NewTradingStrategySignalSourcesDomain(writeDto.SignalSources)
	if sourcesError != nil {
		return TradingStrategyDomain{}, sourcesError
	}

	// Both conditions are required. A set of rules missing one only ever says a
	// single kind of thing, which is not a set of rules that judges anything.
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
		id:            writeDto.ID,
		ownerID:       writeDto.OwnerID,
		name:          name,
		tradingMode:   tradingMode,
		signalSources: signalSources,
		buyCondition:  buyCondition,
		sellCondition: sellCondition,
	}, nil
}

// ToEntity is this trading strategy as the rows it is stored as.
func (tradingStrategyDomain TradingStrategyDomain) ToEntity() entities.TradingStrategy {
	return entities.TradingStrategy{
		ID:            tradingStrategyDomain.id,
		OwnerID:       tradingStrategyDomain.ownerID,
		Name:          tradingStrategyDomain.name,
		TradingMode:   string(tradingStrategyDomain.tradingMode.Value()),
		SignalSources: tradingStrategyDomain.signalSources.ToEntities(),
		ConditionNodes: []entities.TradingStrategyConditionNode{
			tradingStrategyDomain.buyCondition.ToEntity(vo.TradingStrategyConditionSideBuy, 0),
			tradingStrategyDomain.sellCondition.ToEntity(vo.TradingStrategyConditionSideSell, 0),
		},
	}
}
