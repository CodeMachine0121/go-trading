package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotNameMaxLength is how long a bot's name may be, counted after the blanks
// around it are dropped. It matches a strategy script's limit rather than picking a second
// number, because there is no reason the two would ever want to differ and two
// numbers are two things to keep in step.
const strategyBotNameMaxLength = 128

// strategyBotTriggerIntervalMinimumMinutes is the shortest a bot may wait between
// rounds. One minute, because the finest stored candle covers one minute — asking
// more often only fetches the same candle again and reaches the same conclusion.
const strategyBotTriggerIntervalMinimumMinutes = 1

// strategyBotTriggerIntervalMaximumMinutes is the longest. A day: past that, a bot
// is not watching a market, and whoever wants a weekly look is better served by
// opening the screen.
const strategyBotTriggerIntervalMaximumMinutes = 1440

// StrategyBotDomain holds one strategy bot as it is being saved and guarantees its
// own invariants. An instance only exists when every rule passed, so there is no
// half-valid bot.
//
// It delegates the two halves that have rules of their own — the sources and the
// conditions — rather than restating them. That is also what makes the ordering
// here matter: the sources are settled first because the conditions may only name
// labels the sources declared, and the labels are not known until the sources are.
//
// It knows nothing about whether the bot is running. Starting, stopping and being
// halted happen to a bot that already exists and has already passed all of this, so
// they live in StrategyBotRunStateDomain instead — which also means nothing on this
// path can change a run state by accident.
type StrategyBotDomain struct {
	id                     uint
	ownerID                uint
	name                   string
	symbol                 string
	triggerIntervalMinutes int
	signalSources          StrategyBotSignalSourcesDomain
	buyCondition           StrategyBotConditionDomain
	sellCondition          StrategyBotConditionDomain
}

// NewStrategyBotDomain validates the bot against every rule that applies to it. The
// rules are identical whether it is being created or rewritten, because both arrive
// here as the same shape.
func NewStrategyBotDomain(writeDto dto.StrategyBotWriteDto) (StrategyBotDomain, error) {
	// A bot with nobody behind it is refused here rather than at the store, because
	// "every bot has an owner" is a rule about bots, not a constraint that happens
	// to exist on a column.
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

	// Asked of the model every other path already asks, rather than trimmed here.
	// What a bot stores is what its rounds later query with, so a symbol accepted
	// under one set of rules and queried under another finds no candles at all —
	// and a round that finds no candles is recorded as a hold, which reads exactly
	// like a round that concluded hold. The mistake is therefore invisible in the
	// history, which is why it is worth having only one rule about it.
	tradingSymbol, symbolError := NewTradingSymbolDomain(strings.TrimSpace(writeDto.Symbol))
	if symbolError != nil {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須指定這台機器人要盯哪一個交易標的", ErrStrategyBotValidation)
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

	signalSources, sourcesError := NewStrategyBotSignalSourcesDomain(writeDto.SignalSources)
	if sourcesError != nil {
		return StrategyBotDomain{}, sourcesError
	}

	// Both conditions are required. A bot missing one only ever says a single kind
	// of thing, which is not a bot that judges anything.
	buyCondition, buyError := NewStrategyBotConditionDomain(
		writeDto.BuyCondition, signalSources.Labels())
	if buyError != nil {
		return StrategyBotDomain{}, fmt.Errorf("%w（買入條件）", buyError)
	}

	sellCondition, sellError := NewStrategyBotConditionDomain(
		writeDto.SellCondition, signalSources.Labels())
	if sellError != nil {
		return StrategyBotDomain{}, fmt.Errorf("%w（賣出條件）", sellError)
	}

	return StrategyBotDomain{
		id:                     writeDto.ID,
		ownerID:                writeDto.OwnerID,
		name:                   name,
		symbol:                 tradingSymbol.Value(),
		triggerIntervalMinutes: writeDto.TriggerIntervalMinutes,
		signalSources:          signalSources,
		buyCondition:           buyCondition,
		sellCondition:          sellCondition,
	}, nil
}

// ToEntity is this bot as the rows it is stored as, run state and all left at the
// values a newly created bot has.
//
// A rewrite carries the same defaults, and that is correct rather than careless: a
// bot can only be rewritten while it is stopped, so stopped with nothing sent and
// nothing halted is exactly what it already was.
func (strategyBotDomain StrategyBotDomain) ToEntity() entities.StrategyBot {
	return entities.StrategyBot{
		ID:                     strategyBotDomain.id,
		OwnerID:                strategyBotDomain.ownerID,
		Name:                   strategyBotDomain.name,
		Symbol:                 strategyBotDomain.symbol,
		TriggerIntervalMinutes: strategyBotDomain.triggerIntervalMinutes,
		RunState:               string(vo.StrategyBotStopped),
		SignalSources:          strategyBotDomain.signalSources.ToEntities(),
		ConditionNodes: []entities.StrategyBotConditionNode{
			strategyBotDomain.buyCondition.ToEntity(vo.StrategyBotConditionSideBuy, 0),
			strategyBotDomain.sellCondition.ToEntity(vo.StrategyBotConditionSideSell, 0),
		},
	}
}
