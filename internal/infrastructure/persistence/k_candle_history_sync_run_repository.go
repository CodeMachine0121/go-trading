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

type KCandleHistorySyncRunRepository struct {
	database *gorm.DB
}

func NewKCandleHistorySyncRunRepository(database *gorm.DB) *KCandleHistorySyncRunRepository {
	return &KCandleHistorySyncRunRepository{database: database}
}

// Save writes the whole row; a run has a single writer goroutine, and a column list would go stale when fields are added.
func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) Save(
	executionContext context.Context, syncRun entities.KCandleHistorySyncRun,
) (entities.KCandleHistorySyncRun, error) {
	// Counts are named explicitly so zero values are written rather than skipped.
	saved := kCandleHistorySyncRunRepository.database.WithContext(executionContext).
		Select("*").Omit("ID").Save(&syncRun)
	if saved.Error != nil {
		return entities.KCandleHistorySyncRun{},
			kCandleHistorySyncRunRepository.writeFailureOf(saved.Error, syncRun.Symbol)
	}

	return syncRun, nil
}

// writeFailureOf maps a unique-index violation (a concurrent start on the same symbol) to "already running"; any other error stays a fault.
func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) writeFailureOf(
	writeError error, symbol string,
) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == KCandleHistorySyncOneRunningPerSymbolIndex {
		return domains.KCandleHistorySyncInProgress(symbol)
	}

	return fmt.Errorf("save k candle history sync run: %w", writeError)
}

func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.KCandleHistorySyncRun, bool, error) {
	syncRun := entities.KCandleHistorySyncRun{}
	found := kCandleHistorySyncRunRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&syncRun)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.KCandleHistorySyncRun{}, false, nil
	}
	if found.Error != nil {
		return entities.KCandleHistorySyncRun{}, false,
			fmt.Errorf("find k candle history sync run: %w", found.Error)
	}

	return syncRun, true, nil
}

func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) CountRunning(
	executionContext context.Context,
) (int, error) {
	runningCount := int64(0)
	counted := kCandleHistorySyncRunRepository.database.WithContext(executionContext).
		Model(&entities.KCandleHistorySyncRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.KCandleHistorySyncRunning)}).
		Count(&runningCount)
	if counted.Error != nil {
		return 0, fmt.Errorf("count running k candle history sync runs: %w", counted.Error)
	}

	return int(runningCount), nil
}

// FailAllRunning fails every running run in one statement, since all are stale after a restart.
func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) FailAllRunning(
	executionContext context.Context, reason string, finishedAt time.Time,
) (int, error) {
	// The finish time is set too, so a swept run does not look still in flight.
	swept := kCandleHistorySyncRunRepository.database.WithContext(executionContext).
		Model(&entities.KCandleHistorySyncRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.KCandleHistorySyncRunning)}).
		Select("status", "failure_reason", "finished_at").
		Updates(entities.KCandleHistorySyncRun{
			Status:        string(vo.KCandleHistorySyncFailed),
			FailureReason: reason,
			FinishedAt:    &finishedAt,
		})
	if swept.Error != nil {
		return 0, fmt.Errorf("fail running k candle history sync runs: %w", swept.Error)
	}

	return int(swept.RowsAffected), nil
}
