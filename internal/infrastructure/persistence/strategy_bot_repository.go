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

// StrategyBotRepository stores bots; their rules live with the trading strategy they reference.
type StrategyBotRepository struct {
	database *gorm.DB
}

func NewStrategyBotRepository(database *gorm.DB) *StrategyBotRepository {
	return &StrategyBotRepository{database: database}
}

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

	// Named columns keep a rewrite from touching lifecycle fields or the immutable market kind, and make empty values written rather than skipped.
	updates := strategyBotRepository.database.WithContext(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: botRow.ID}).
		Select(
			"name", "symbol", "trading_strategy_id", "trigger_interval_minutes",
			"position_plan_capital", "position_plan_sizing_mode",
			"position_plan_sizing_value",
			"position_plan_stop_loss_percentage", "position_plan_take_profit_percentage",
			"position_plan_contract_leverage").
		Updates(entities.StrategyBot{
			Name:                             botRow.Name,
			Symbol:                           botRow.Symbol,
			TradingStrategyID:                botRow.TradingStrategyID,
			TriggerIntervalMinutes:           botRow.TriggerIntervalMinutes,
			PositionPlanCapital:              botRow.PositionPlanCapital,
			PositionPlanSizingMode:           botRow.PositionPlanSizingMode,
			PositionPlanSizingValue:          botRow.PositionPlanSizingValue,
			PositionPlanStopLossPercentage:   botRow.PositionPlanStopLossPercentage,
			PositionPlanTakeProfitPercentage: botRow.PositionPlanTakeProfitPercentage,
			PositionPlanLeverage:             botRow.PositionPlanLeverage,
		})
	if updates.Error != nil {
		return entities.StrategyBot{}, strategyBotRepository.writeFailureOf(updates.Error, bot.Name)
	}

	return strategyBotRepository.FindOne(executionContext, botRow.ID)
}

// StrategyBotNameIndex enforces unique bot names per owner; the write path recognises its violation as a name conflict.
const StrategyBotNameIndex = "idx_strategy_bots_owner_name"

// writeFailureOf maps a name-index violation to a name conflict; any other error stays a fault.
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

func (strategyBotRepository *StrategyBotRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.StrategyBot, error) {
	bot := entities.StrategyBot{}

	// A string condition, because GORM drops zero-valued struct fields and ID zero would match the first bot.
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

// FindAllByTradingStrategy does not filter by owner, since a strategy already has exactly one owner who alone can reference it.
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

// Delete cascades to the bot's runs but leaves its trading strategy, which other bots may use.
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

// UpdateRunState writes only lifecycle columns, so a finished round cannot alter the bot's configuration.
func (strategyBotRepository *StrategyBotRepository) UpdateRunState(
	executionContext context.Context, bot entities.StrategyBot,
) error {
	result := strategyBotRepository.database.WithContext(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: bot.ID}).
		Select("run_state", "next_run_at", "last_sent_signal", "halt_reason", "conflicting").
		// UpdateColumns rather than Updates, which would bump UpdatedAt on every round even though nothing was modified.
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

// FindDue returns at most limit due running bots, oldest due first.
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
