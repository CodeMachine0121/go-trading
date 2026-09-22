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

// KCandleContractHistorySyncRunRepository stores contract history syncs in PostgreSQL.
type KCandleContractHistorySyncRunRepository struct {
	database *gorm.DB
}

func NewKCandleContractHistorySyncRunRepository(
	database *gorm.DB,
) *KCandleContractHistorySyncRunRepository {
	return &KCandleContractHistorySyncRunRepository{database: database}
}

// Save writes a run whole, whether it is being created or brought up to date. A run
// is written by one goroutine and nobody else, so there is no other writer for a full
// row to overwrite.
func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) Save(
	executionContext context.Context, syncRun entities.KCandleContractHistorySyncRun,
) (entities.KCandleContractHistorySyncRun, error) {
	// Zero is a real answer for every count here — nothing stored, nothing skipped,
	// no chunk finished yet — so the columns are named rather than left to the ORM's
	// reading of an empty value.
	saved := kCandleContractHistorySyncRunRepository.database.WithContext(executionContext).
		Select("*").Omit("ID").Save(&syncRun)
	if saved.Error != nil {
		return entities.KCandleContractHistorySyncRun{},
			kCandleContractHistorySyncRunRepository.writeFailureOf(saved.Error, syncRun.Symbol)
	}

	return syncRun, nil
}

// writeFailureOf tells a person asking twice apart from something actually going wrong.
//
// One broken index means a second run was starting on a symbol that already had one —
// two requests that both read "nothing in flight" before either had written, which is
// why the database is what decides rather than a read here.
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

// FindOne answers with the run carrying this identifier, and whether there is one.
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

// FailAllRunning marks every run still recorded as running as failed.
//
// It is one statement rather than a read followed by writes, because there is no
// decision to make per row: every one of them is stale by definition, since the
// process that was fetching no longer exists.
func (kCandleContractHistorySyncRunRepository *KCandleContractHistorySyncRunRepository) FailAllRunning(
	executionContext context.Context, reason string, finishedAt time.Time,
) (int, error) {
	// The finish time is written along with the status. A swept run left with none
	// would read as "failed but still going" to anything using that column to tell a
	// run in flight from one that is over.
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
