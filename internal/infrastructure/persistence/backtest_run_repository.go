package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BacktestRunRepository reads through ambientTransactionDatabase so a run's ending can land in one transaction with its report.
type BacktestRunRepository struct {
	database ambientTransactionDatabase
}

func NewBacktestRunRepository(database *gorm.DB) *BacktestRunRepository {
	return &BacktestRunRepository{database: ambientTransactionDatabase{root: database}}
}

func (backtestRunRepository *BacktestRunRepository) Save(
	executionContext context.Context, backtestRun entities.BacktestRun,
) (entities.BacktestRun, error) {
	// Counts are named explicitly so zero values are written rather than skipped.
	saved := backtestRunRepository.database.within(executionContext).
		Omit("Owner", "Result").Select("*").Omit("ID").Save(&backtestRun)
	if saved.Error != nil {
		return entities.BacktestRun{}, fmt.Errorf("save backtest run: %w", saved.Error)
	}

	return backtestRun, nil
}

func (backtestRunRepository *BacktestRunRepository) FindOneOwned(
	executionContext context.Context, ownerID uint, id uint,
) (entities.BacktestRun, bool, error) {
	backtestRun := entities.BacktestRun{}
	found := backtestRunRepository.database.within(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		First(&backtestRun)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.BacktestRun{}, false, nil
	}
	if found.Error != nil {
		return entities.BacktestRun{}, false, fmt.Errorf("find backtest run: %w", found.Error)
	}

	return backtestRun, true, nil
}

func (backtestRunRepository *BacktestRunRepository) FindPageByOwner(
	executionContext context.Context, ownerID uint, query dto.BacktestRunListQueryDto,
) ([]entities.BacktestRun, int64, error) {
	matching := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID})
	if query.Status != "" {
		matching = matching.Where(clause.Eq{Column: "status", Value: query.Status})
	}
	if query.StrategyScriptID != 0 {
		matching = matching.Where(clause.Eq{Column: "strategy_script_id", Value: query.StrategyScriptID})
	}
	if query.TradingStrategyID != 0 {
		matching = matching.Where(clause.Eq{Column: "trading_strategy_id", Value: query.TradingStrategyID})
	}

	totalCount := int64(0)
	if countError := matching.Session(&gorm.Session{}).Count(&totalCount).Error; countError != nil {
		return nil, 0, fmt.Errorf("count backtest runs: %w", countError)
	}

	backtestRuns := []entities.BacktestRun{}
	listed := matching.Session(&gorm.Session{}).
		Order("id DESC").
		Limit(query.Limit).
		Find(&backtestRuns)
	if listed.Error != nil {
		return nil, 0, fmt.Errorf("list backtest runs: %w", listed.Error)
	}

	return backtestRuns, totalCount, nil
}

func (backtestRunRepository *BacktestRunRepository) CountActiveByOwner(
	executionContext context.Context, ownerID uint,
) (int, error) {
	activeCount := int64(0)
	counted := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.IN{Column: "status", Values: []any{
			string(vo.BacktestRunQueued), string(vo.BacktestRunRunning),
		}}).
		Count(&activeCount)
	if counted.Error != nil {
		return 0, fmt.Errorf("count active backtest runs: %w", counted.Error)
	}

	return int(activeCount), nil
}

// LockOldestQueued must run inside a transaction, which holds the row lock until the claim is saved.
func (backtestRunRepository *BacktestRunRepository) LockOldestQueued(
	executionContext context.Context,
) (entities.BacktestRun, bool, error) {
	queued := entities.BacktestRun{}
	picked := backtestRunRepository.database.within(executionContext).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked}).
		Where(clause.Eq{Column: "status", Value: string(vo.BacktestRunQueued)}).
		Order("id ASC").
		First(&queued)
	if errors.Is(picked.Error, gorm.ErrRecordNotFound) {
		return entities.BacktestRun{}, false, nil
	}
	if picked.Error != nil {
		return entities.BacktestRun{}, false, fmt.Errorf("lock queued backtest run: %w", picked.Error)
	}

	return queued, true, nil
}

func (backtestRunRepository *BacktestRunRepository) SaveClaim(
	executionContext context.Context, backtestRun entities.BacktestRun,
) error {
	claimed := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "id", Value: backtestRun.ID}).
		Select("status", "replica_name", "started_at").
		UpdateColumns(entities.BacktestRun{
			Status: backtestRun.Status, ReplicaName: backtestRun.ReplicaName, StartedAt: backtestRun.StartedAt,
		})
	if claimed.Error != nil {
		return fmt.Errorf("claim queued backtest run: %w", claimed.Error)
	}

	return nil
}

// UpdateIfRunning only touches a row still running under the same replica, so a cancel or a sweep is never overwritten by a late write.
func (backtestRunRepository *BacktestRunRepository) UpdateIfRunning(
	executionContext context.Context, backtestRun entities.BacktestRun,
) (bool, error) {
	updated := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "id", Value: backtestRun.ID}).
		Where(clause.Eq{Column: "status", Value: string(vo.BacktestRunRunning)}).
		Where(clause.Eq{Column: "replica_name", Value: backtestRun.ReplicaName}).
		Select("status", "completed_bar_count", "total_bar_count", "failure_reason",
			"total_return_rate", "maximum_drawdown", "trade_count", "finished_at").
		Updates(map[string]any{
			"status":              backtestRun.Status,
			"completed_bar_count": backtestRun.CompletedBarCount,
			"total_bar_count":     backtestRun.TotalBarCount,
			"failure_reason":      backtestRun.FailureReason,
			"total_return_rate":   backtestRun.TotalReturnRate,
			"maximum_drawdown":    backtestRun.MaximumDrawdown,
			"trade_count":         backtestRun.TradeCount,
			"finished_at":         backtestRun.FinishedAt,
		})
	if updated.Error != nil {
		return false, fmt.Errorf("update running backtest run: %w", updated.Error)
	}

	return updated.RowsAffected == 1, nil
}

// CancelIfActive ends the run only while it is queued or running; the runner then sees its next progress write refused and stops.
func (backtestRunRepository *BacktestRunRepository) CancelIfActive(
	executionContext context.Context, backtestRun entities.BacktestRun,
) (bool, error) {
	cancelled := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "id", Value: backtestRun.ID}).
		Where(clause.Eq{Column: "owner_id", Value: backtestRun.OwnerID}).
		Where(clause.IN{Column: "status", Values: []any{
			string(vo.BacktestRunQueued), string(vo.BacktestRunRunning),
		}}).
		Select("status", "finished_at").
		UpdateColumns(entities.BacktestRun{Status: backtestRun.Status, FinishedAt: backtestRun.FinishedAt})
	if cancelled.Error != nil {
		return false, fmt.Errorf("cancel backtest run: %w", cancelled.Error)
	}

	return cancelled.RowsAffected == 1, nil
}

// FailRunningOutside fails, in one statement, every running run left by a replica no longer alive; queued runs are left for the living.
func (backtestRunRepository *BacktestRunRepository) FailRunningOutside(
	executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
) (int, error) {
	liveReplicaValues := make([]any, 0, len(liveReplicaNames))
	for _, liveReplicaName := range liveReplicaNames {
		liveReplicaValues = append(liveReplicaValues, liveReplicaName)
	}
	finishedAtUtc := finishedAt.UTC()

	swept := backtestRunRepository.database.within(executionContext).
		Model(&entities.BacktestRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.BacktestRunRunning)}).
		Where(clause.Not(clause.IN{Column: "replica_name", Values: liveReplicaValues})).
		Select("status", "failure_reason", "finished_at").
		Updates(entities.BacktestRun{
			Status: string(vo.BacktestRunFailed), FailureReason: reason, FinishedAt: &finishedAtUtc,
		})
	if swept.Error != nil {
		return 0, fmt.Errorf("fail running backtest runs: %w", swept.Error)
	}

	return int(swept.RowsAffected), nil
}

// DeleteFinishedBefore removes ended runs with their reports; the report row goes with the run by cascade.
func (backtestRunRepository *BacktestRunRepository) DeleteFinishedBefore(
	executionContext context.Context, cutoff time.Time,
) (int, error) {
	deleted := backtestRunRepository.database.within(executionContext).
		Where(clause.Lt{Column: "finished_at", Value: cutoff.UTC()}).
		Delete(&entities.BacktestRun{})
	if deleted.Error != nil {
		return 0, fmt.Errorf("delete expired backtest runs: %w", deleted.Error)
	}

	return int(deleted.RowsAffected), nil
}
