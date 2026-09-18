package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StrategyBotRepository stores strategy bots in PostgreSQL. The rules a bot follows
// are stored by TradingStrategyRepository; a bot row only names which set it points
// at.
type StrategyBotRepository struct {
	database *gorm.DB
}

func NewStrategyBotRepository(database *gorm.DB) *StrategyBotRepository {
	return &StrategyBotRepository{database: database}
}

// Save stores this bot, replacing whatever it had before.
//
// There are no children to clear and rewrite any more: the sources and the two
// condition trees moved to the trading strategy this bot names, so a bot is one row.
func (strategyBotRepository *StrategyBotRepository) Save(
	executionContext context.Context, bot entities.StrategyBot,
) (entities.StrategyBot, error) {
	botRow := bot
	botRow.Owner = entities.User{}
	botRow.TradingStrategy = entities.TradingStrategy{}
	botRow.RunRecords = nil

	if botRow.ID == 0 {
		createError := strategyBotRepository.database.WithContext(executionContext).
			Omit(clause.Associations).Create(&botRow).Error
		if createError != nil {
			return entities.StrategyBot{}, strategyBotRepository.writeFailureOf(createError, bot.Name)
		}

		return strategyBotRepository.FindOne(executionContext, botRow.ID)
	}

	// The columns are named so that a rewrite cannot reach the ones a bot's life
	// owns — a run state, a next round, a last sent signal. Naming them also makes
	// an empty value mean empty rather than "unchanged", which is how GORM reads a
	// struct otherwise.
	updates := strategyBotRepository.database.WithContext(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: botRow.ID}).
		Select(
			"name", "symbol", "trading_strategy_id", "trigger_interval_minutes",
			"position_plan_capital", "position_plan_sizing_mode",
			"position_plan_sizing_value", "position_plan_leverage",
			"position_plan_stop_loss_percentage", "position_plan_take_profit_percentage").
		Updates(entities.StrategyBot{
			Name:                             botRow.Name,
			Symbol:                           botRow.Symbol,
			TradingStrategyID:                botRow.TradingStrategyID,
			TriggerIntervalMinutes:           botRow.TriggerIntervalMinutes,
			PositionPlanCapital:              botRow.PositionPlanCapital,
			PositionPlanSizingMode:           botRow.PositionPlanSizingMode,
			PositionPlanSizingValue:          botRow.PositionPlanSizingValue,
			PositionPlanLeverage:             botRow.PositionPlanLeverage,
			PositionPlanStopLossPercentage:   botRow.PositionPlanStopLossPercentage,
			PositionPlanTakeProfitPercentage: botRow.PositionPlanTakeProfitPercentage,
		})
	if updates.Error != nil {
		return entities.StrategyBot{}, strategyBotRepository.writeFailureOf(updates.Error, bot.Name)
	}

	return strategyBotRepository.FindOne(executionContext, botRow.ID)
}

// StrategyBotNameIndex is the index that makes a bot's name unique within its
// owner's collection. It is named here because the write path has to recognise this
// one breaking specifically — any other broken constraint is a fault, not a person
// reusing a name.
const StrategyBotNameIndex = "idx_strategy_bots_owner_name"

// writeFailureOf turns a failed write into the refusal it actually is. A broken name
// index is a person reusing a name they already have; anything else is a fault, and
// dressing it up as a name conflict would send them off renaming a bot that was
// never the problem.
func (strategyBotRepository *StrategyBotRepository) writeFailureOf(
	writeError error, name string,
) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == StrategyBotNameIndex {
		return fmt.Errorf("%w: 機器人名稱「%s」已被使用", domains.ErrStrategyBotNameConflict, name)
	}

	return fmt.Errorf("save strategy bot: %w", writeError)
}

// FindOne returns this bot, with the name of the trading strategy it follows.
func (strategyBotRepository *StrategyBotRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.StrategyBot, error) {
	bot := entities.StrategyBot{}

	// The condition is spelled out rather than given as a struct, because GORM
	// drops zero-valued struct fields — and an identifier of nothing would become
	// no condition at all, handing back whichever bot happens to be first.
	result := strategyBotRepository.database.WithContext(executionContext).
		Preload("TradingStrategy").
		Where(clause.Eq{Column: "id", Value: id}).
		First(&bot)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.StrategyBot{}, domains.StrategyBotNotFound(id)
	}
	if result.Error != nil {
		return entities.StrategyBot{}, fmt.Errorf("find strategy bot: %w", result.Error)
	}

	return bot, nil
}

// FindAllByOwner returns this person's bots, by name.
func (strategyBotRepository *StrategyBotRepository) FindAllByOwner(
	executionContext context.Context, ownerID uint,
) ([]entities.StrategyBot, error) {
	bots := []entities.StrategyBot{}

	result := strategyBotRepository.database.WithContext(executionContext).
		Preload("TradingStrategy").
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("list strategy bots: %w", result.Error)
	}

	return bots, nil
}

// FindAllByTradingStrategy returns every bot following this set of rules.
//
// It does not filter by owner. A trading strategy already belongs to exactly one
// person and only they can point a bot at it, so an owner clause here would narrow
// nothing and would quietly become the place a future sharing feature goes wrong.
func (strategyBotRepository *StrategyBotRepository) FindAllByTradingStrategy(
	executionContext context.Context, tradingStrategyID uint,
) ([]entities.StrategyBot, error) {
	bots := []entities.StrategyBot{}

	result := strategyBotRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyID}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy bots by trading strategy: %w", result.Error)
	}

	return bots, nil
}

// Delete removes this bot. Its rounds go with it by cascade; the trading strategy it
// followed is untouched — that is a thing of its own, and other bots may use it.
func (strategyBotRepository *StrategyBotRepository) Delete(
	executionContext context.Context, id uint,
) error {
	result := strategyBotRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Delete(&entities.StrategyBot{})
	if result.Error != nil {
		return fmt.Errorf("delete strategy bot: %w", result.Error)
	}

	return nil
}

// UpdateRunState writes only the columns a bot's life touches.
//
// Naming them is the point. A round finishing must not be able to alter a condition,
// and the narrow write is what makes that true of the code rather than of its
// author's intentions.
func (strategyBotRepository *StrategyBotRepository) UpdateRunState(
	executionContext context.Context, bot entities.StrategyBot,
) error {
	result := strategyBotRepository.database.WithContext(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: bot.ID}).
		Select("run_state", "next_run_at", "last_sent_signal", "halt_reason", "conflicting").
		// UpdateColumns rather than Updates, because Updates also touches the
		// auto-managed UpdatedAt even under a Select. A bot's last-modified time is
		// handed to its owner next to its creation time; advancing it every
		// trigger interval, for ever, with nobody having modified anything, leaves
		// the field saying nothing at all.
		UpdateColumns(entities.StrategyBot{
			RunState:       bot.RunState,
			NextRunAt:      bot.NextRunAt,
			LastSentSignal: bot.LastSentSignal,
			HaltReason:     bot.HaltReason,
			Conflicting:    bot.Conflicting,
		})
	if result.Error != nil {
		return fmt.Errorf("update strategy bot run state: %w", result.Error)
	}

	return nil
}

// CountRunningByOwner is how many of this person's bots are running.
func (strategyBotRepository *StrategyBotRepository) CountRunningByOwner(
	executionContext context.Context, ownerID uint,
) (int, error) {
	runningBotCount := int64(0)

	result := strategyBotRepository.database.WithContext(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "run_state", Value: string(vo.StrategyBotRunning)}).
		Count(&runningBotCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count running strategy bots: %w", result.Error)
	}

	return int(runningBotCount), nil
}

// FindDue returns running bots that are due, oldest due first and at most this many.
func (strategyBotRepository *StrategyBotRepository) FindDue(
	executionContext context.Context, moment time.Time, limit int,
) ([]entities.StrategyBot, error) {
	bots := []entities.StrategyBot{}

	result := strategyBotRepository.database.WithContext(executionContext).
		Preload("TradingStrategy").
		Where(clause.Eq{Column: "run_state", Value: string(vo.StrategyBotRunning)}).
		Where(clause.Lte{Column: "next_run_at", Value: moment.UTC()}).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("find due strategy bots: %w", result.Error)
	}

	return bots, nil
}
