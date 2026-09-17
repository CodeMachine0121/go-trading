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

// KCandleHistorySyncRunRepository stores history syncs in PostgreSQL.
type KCandleHistorySyncRunRepository struct {
	database *gorm.DB
}

func NewKCandleHistorySyncRunRepository(database *gorm.DB) *KCandleHistorySyncRunRepository {
	return &KCandleHistorySyncRunRepository{database: database}
}

// Save writes a run whole, whether it is being created or brought up to date.
//
// The whole row goes every time rather than only the columns that moved. A run is
// written by one goroutine and nobody else, so there is no other writer for a full
// row to overwrite, and the alternative — a list of columns kept in step with the
// fields — is a list that goes stale the first time a field is added.
func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) Save(
	executionContext context.Context, syncRun entities.KCandleHistorySyncRun,
) (entities.KCandleHistorySyncRun, error) {
	// Zero is a real answer for every count here — nothing stored, nothing skipped,
	// no chunk finished yet — so the columns are named rather than left to GORM's
	// reading of an empty value.
	saved := kCandleHistorySyncRunRepository.database.WithContext(executionContext).
		Select("*").Omit("ID").Save(&syncRun)
	if saved.Error != nil {
		return entities.KCandleHistorySyncRun{},
			kCandleHistorySyncRunRepository.writeFailureOf(saved.Error, syncRun.Symbol)
	}

	return syncRun, nil
}

// writeFailureOf tells a person asking twice apart from something actually going wrong.
//
// One broken index means a second run was starting on a symbol that already had one —
// two requests that both read "nothing in flight" before either had written, which is
// why the database is what decides rather than a read here. Anything else is a fault,
// and dressing it up as "one is already running" would leave somebody waiting on a run
// that was never started.
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

// FindOne answers with the run carrying this identifier, and whether there is one.
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

// FailAllRunning marks every run still recorded as running as failed.
//
// It is one statement rather than a read followed by writes, because there is no
// decision to make per row: every one of them is stale by definition, since the
// process that was fetching no longer exists.
func (kCandleHistorySyncRunRepository *KCandleHistorySyncRunRepository) FailAllRunning(
	executionContext context.Context, reason string, finishedAt time.Time,
) (int, error) {
	// The finish time is written along with the status. A swept run left with none
	// would read as "failed but still going" to anything using that column to tell
	// a run in flight from one that is over — which is exactly what it is for.
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
