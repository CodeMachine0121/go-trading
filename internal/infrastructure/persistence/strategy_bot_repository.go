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

// StrategyBotRepository stores strategy bots, their signal sources and their two
// condition trees, in PostgreSQL.
type StrategyBotRepository struct {
	database *gorm.DB
}

func NewStrategyBotRepository(database *gorm.DB) *StrategyBotRepository {
	return &StrategyBotRepository{database: database}
}

// Save stores this bot whole, replacing whatever it had before.
//
// Everything happens in one transaction, because a bot whose conditions were
// replaced but whose sources were not is a bot that can name a label that no longer
// exists — and it would then run that way every few minutes.
//
// The children are cleared and written again rather than compared and patched. A
// condition tree holds at most thirty-two nodes and is only ever read and written
// whole; the reads a diff would save are worth less than the ways a diff can be
// wrong.
func (strategyBotRepository *StrategyBotRepository) Save(
	executionContext context.Context, bot entities.StrategyBot,
) (entities.StrategyBot, error) {
	savedBot := bot

	transactionError := strategyBotRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The bot row goes first and alone: its identifier is what every child
			// row needs, and on a create nobody knows it until this returns.
			// Omitting the associations is what stops GORM writing them here in a
			// shape this code would then have to undo.
			botRow := bot
			botRow.SignalSources = nil
			botRow.ConditionNodes = nil
			botRow.RunRecords = nil

			if botRow.ID == 0 {
				if createError := transaction.Omit(clause.Associations).Create(&botRow).Error; createError != nil {
					return createError
				}
			} else {
				// The columns are named so that a rewrite cannot reach the ones a
				// bot's life owns — a run state, a next round, a last sent signal.
				// Naming them also makes an empty value mean empty rather than
				// "unchanged", which is how GORM reads a struct otherwise.
				updates := transaction.Model(&entities.StrategyBot{}).
					Where(clause.Eq{Column: "id", Value: botRow.ID}).
					Select("name", "symbol", "trigger_interval_minutes").
					Updates(entities.StrategyBot{
						Name:                   botRow.Name,
						Symbol:                 botRow.Symbol,
						TriggerIntervalMinutes: botRow.TriggerIntervalMinutes,
					})
				if updates.Error != nil {
					return updates.Error
				}

				if deleteError := transaction.
					Where(clause.Eq{Column: "strategy_bot_id", Value: botRow.ID}).
					Delete(&entities.StrategyBotSignalSource{}).Error; deleteError != nil {
					return deleteError
				}

				// Deleting the roots takes their descendants with them through the
				// node table's own cascade, so this does not walk the tree — and
				// therefore cannot walk it wrong.
				if deleteError := transaction.
					Where(clause.Eq{Column: "strategy_bot_id", Value: botRow.ID}).
					Delete(&entities.StrategyBotConditionNode{}).Error; deleteError != nil {
					return deleteError
				}
			}

			for index := range bot.SignalSources {
				signalSource := bot.SignalSources[index]
				signalSource.ID = 0
				signalSource.StrategyBotID = botRow.ID
				for valueIndex := range signalSource.ParameterValues {
					signalSource.ParameterValues[valueIndex].ID = 0
					signalSource.ParameterValues[valueIndex].StrategyBotSignalSourceID = 0
				}

				if createError := transaction.Create(&signalSource).Error; createError != nil {
					return createError
				}
			}

			for index := range bot.ConditionNodes {
				if writeError := strategyBotRepository.writeConditionSubtree(
					transaction, botRow.ID, nil, bot.ConditionNodes[index]); writeError != nil {
					return writeError
				}
			}

			savedBot = botRow

			return nil
		})
	if transactionError != nil {
		return entities.StrategyBot{}, strategyBotRepository.writeFailureOf(transactionError, bot.Name)
	}

	return strategyBotRepository.FindOne(executionContext, savedBot.ID)
}

// writeConditionSubtree writes one node and everything under it, handing each child
// the identifier its parent has just been given.
//
// It descends explicitly rather than relying on the store to write a nested
// association for it. One level of nesting is something an ORM will do; five is
// something to find out about at three in the morning.
func (strategyBotRepository *StrategyBotRepository) writeConditionSubtree(
	transaction *gorm.DB, strategyBotID uint, parentID *uint,
	node entities.StrategyBotConditionNode,
) error {
	children := node.Children

	nodeRow := node
	nodeRow.ID = 0
	nodeRow.StrategyBotID = strategyBotID
	nodeRow.ParentID = parentID
	nodeRow.Children = nil
	nodeRow.Parent = nil

	if createError := transaction.Omit(clause.Associations).Create(&nodeRow).Error; createError != nil {
		return createError
	}

	for index := range children {
		if writeError := strategyBotRepository.writeConditionSubtree(
			transaction, strategyBotID, &nodeRow.ID, children[index]); writeError != nil {
			return writeError
		}
	}

	return nil
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

// FindOne returns this bot with its sources and both trees.
func (strategyBotRepository *StrategyBotRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.StrategyBot, error) {
	bot := entities.StrategyBot{}

	// The condition is spelled out rather than given as a struct, because GORM
	// drops zero-valued struct fields — and an identifier of nothing would become
	// no condition at all, handing back whichever bot happens to be first.
	result := strategyBotRepository.database.WithContext(executionContext).
		Preload("SignalSources.ParameterValues").
		Preload("SignalSources").
		Preload("ConditionNodes").
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
		Preload("SignalSources.ParameterValues").
		Preload("SignalSources").
		Preload("ConditionNodes").
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("list strategy bots: %w", result.Error)
	}

	return bots, nil
}

// Delete removes this bot. Its sources and condition nodes go with it by cascade.
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
		Preload("SignalSources.ParameterValues").
		Preload("SignalSources").
		Preload("ConditionNodes").
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
