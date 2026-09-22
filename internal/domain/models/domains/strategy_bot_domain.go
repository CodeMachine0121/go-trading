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
// It says nothing about the rules the bot follows — it only checks that exactly one
// set of them was named. Whether those rules are this person's, and whether they
// hold together, is TradingStrategyDomain's question, asked once where the rules
// live rather than again in every machine that follows them.
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
	tradingStrategyID      uint
	triggerIntervalMinutes int
	positionPlan           PositionPlanDomain
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

	// Settled beside the interval, because both are about how this machine runs
	// rather than about whose rules it follows. Its own model owns every figure:
	// leaving the group empty is not a failure, and how much one opening stakes is
	// read by the very model a replay reads — in the same words, so a bot and a
	// replay cannot end up disagreeing about what a percentage of a hundred and
	// fifty means.
	positionPlan, positionPlanError := NewPositionPlanDomain(writeDto.PositionPlan)
	if positionPlanError != nil {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: %s", ErrStrategyBotValidation, positionPlanError)
	}

	// Exactly one set of rules, named rather than given. Nothing is refused here
	// for being somebody else's — that answer has to come from reading it, and
	// reading it is the application's job.
	if writeDto.TradingStrategyID == 0 {
		return StrategyBotDomain{}, fmt.Errorf(
			"%w: 必須指名這台機器人要用哪一份交易策略", ErrStrategyBotValidation)
	}

	// What a bot may suggest borrowing is limited by what its rules may borrow. The
	// two questions were answered in two places until now — a replay refused a spot
	return StrategyBotDomain{
		id:                     writeDto.ID,
		ownerID:                writeDto.OwnerID,
		name:                   name,
		symbol:                 tradingSymbol.Value(),
		tradingStrategyID:      writeDto.TradingStrategyID,
		triggerIntervalMinutes: writeDto.TriggerIntervalMinutes,
		positionPlan:           positionPlan,
	}, nil
}

// ToEntity is this bot as the rows it is stored as, run state and all left at the
// values a newly created bot has.
//
// A rewrite carries the same defaults, and that is correct rather than careless: a
// bot can only be rewritten while it is stopped, so stopped with nothing sent and
// nothing halted is exactly what it already was.
func (strategyBotDomain StrategyBotDomain) ToEntity() entities.StrategyBot {
	positionPlanSettings := strategyBotDomain.positionPlan.ToSettingsDto()

	return entities.StrategyBot{
		ID:                               strategyBotDomain.id,
		OwnerID:                          strategyBotDomain.ownerID,
		Name:                             strategyBotDomain.name,
		Symbol:                           strategyBotDomain.symbol,
		TradingStrategyID:                strategyBotDomain.tradingStrategyID,
		TriggerIntervalMinutes:           strategyBotDomain.triggerIntervalMinutes,
		PositionPlanCapital:              positionPlanSettings.Capital,
		PositionPlanSizingMode:           positionPlanSettings.SizingMode,
		PositionPlanSizingValue:          positionPlanSettings.SizingValue,
		PositionPlanStopLossPercentage:   positionPlanSettings.StopLossPercentage,
		PositionPlanTakeProfitPercentage: positionPlanSettings.TakeProfitPercentage,
		RunState:                         string(vo.StrategyBotStopped),
	}
}
