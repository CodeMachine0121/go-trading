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

type KCandleContractHistorySyncRunRepository struct {
	database *gorm.DB
}

func NewKCandleContractHistorySyncRunRepository(
	database *gorm.DB,
) *KCandleContractHistorySyncRunRepository {
	return &KCandleContractHistorySyncRunRepository{database: database}
}

// Save writes the whole row; a run has a single writer goroutine, so nothing else can be overwritten.
func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) Save(
	executionContext context.Context, syncRun entities.KCandleContractHistorySyncRun,
) (entities.KCandleContractHistorySyncRun, error) {
	// Counts are named explicitly so zero values are written rather than skipped.
	saved := kCandleContractHistorySyncRunRepository.database.WithContext(executionContext).
		Select("*").Omit("ID").Save(&syncRun)
	if saved.Error != nil {
		return entities.KCandleContractHistorySyncRun{},
			kCandleContractHistorySyncRunRepository.writeFailureOf(saved.Error, syncRun.Symbol)
	}

	return syncRun, nil
}

// writeFailureOf maps a unique-index violation (a concurrent start on the same symbol) to "already running"; the database, not a prior read, decides.
func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) writeFailureOf(
	writeError error, symbol string,
) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == KCandleContractHistorySyncOneRunningPerSymbolIndex {
		return domains.KCandleHistorySyncInProgress(symbol)
	}

	return fmt.Errorf("save contract k candle history sync run: %w", writeError)
}

func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.KCandleContractHistorySyncRun, bool, error) {
	syncRun := entities.KCandleContractHistorySyncRun{}

	found := kCandleContractHistorySyncRunRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&syncRun)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.KCandleContractHistorySyncRun{}, false, nil
	}
	if found.Error != nil {
		return entities.KCandleContractHistorySyncRun{}, false,
			fmt.Errorf("find contract k candle history sync run: %w", found.Error)
	}

	return syncRun, true, nil
}

func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) CountRunning(
	executionContext context.Context,
) (int, error) {
	runningCount := int64(0)
	counted := kCandleContractHistorySyncRunRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContractHistorySyncRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.KCandleHistorySyncRunning)}).
		Count(&runningCount)
	if counted.Error != nil {
		return 0, fmt.Errorf("count running contract k candle history sync runs: %w", counted.Error)
	}

	return int(runningCount), nil
}

// FailAllRunning fails every running run in one statement, since all are stale after a restart.
func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) FailAllRunning(
	executionContext context.Context, reason string, finishedAt time.Time,
) (int, error) {
	// The finish time is set too, so a swept run does not look still in flight.
	swept := kCandleContractHistorySyncRunRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContractHistorySyncRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.KCandleHistorySyncRunning)}).
		Select("status", "failure_reason", "finished_at").
		Updates(entities.KCandleContractHistorySyncRun{
			Status:        string(vo.KCandleHistorySyncFailed),
			FailureReason: reason,
			FinishedAt:    &finishedAt,
		})
	if swept.Error != nil {
		return 0, fmt.Errorf("fail running contract k candle history sync runs: %w", swept.Error)
	}

	return int(swept.RowsAffected), nil
}
