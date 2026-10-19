package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var symbolScreeningActiveStatuses = []any{string(vo.SymbolScreeningRunQueued), string(vo.SymbolScreeningRunRunning)}

type SymbolScreeningRunRepository struct {
	database ambientTransactionDatabase
}

func NewSymbolScreeningRunRepository(database *gorm.DB) *SymbolScreeningRunRepository {
	return &SymbolScreeningRunRepository{database: ambientTransactionDatabase{root: database}}
}

func (runRepository *SymbolScreeningRunRepository) Save(
	executionContext context.Context, run entities.SymbolScreeningRun,
) (entities.SymbolScreeningRun, error) {
	saved := runRepository.database.within(executionContext).Omit("Owner", "Profile").Create(&run)
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](saved.Error)
	if isPostgresError && postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == SymbolScreeningOneActiveRunPerOwnerIndex {
		return entities.SymbolScreeningRun{}, domains.ErrSymbolScreeningRunAlreadyActive
	}
	if saved.Error != nil {
		return entities.SymbolScreeningRun{}, fmt.Errorf("save symbol screening run: %w", saved.Error)
	}

	return run, nil
}

func (runRepository *SymbolScreeningRunRepository) FindOneOwned(
	executionContext context.Context, ownerID uint, id uint,
) (entities.SymbolScreeningRun, bool, error) {
	run := entities.SymbolScreeningRun{}
	found := runRepository.database.within(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		First(&run)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.SymbolScreeningRun{}, false, nil
	}
	if found.Error != nil {
		return entities.SymbolScreeningRun{}, false, fmt.Errorf("find symbol screening run: %w", found.Error)
	}

	return run, true, nil
}

func (runRepository *SymbolScreeningRunRepository) FindPageByOwner(
	executionContext context.Context, ownerID uint, query dto.SymbolScreeningRunListQueryDto,
) ([]entities.SymbolScreeningRun, int64, error) {
	matching := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID})
	if query.Status != "" {
		matching = matching.Where(clause.Eq{Column: "status", Value: query.Status})
	}

	totalCount := int64(0)
	if countError := matching.Session(&gorm.Session{}).Count(&totalCount).Error; countError != nil {
		return nil, 0, fmt.Errorf("count symbol screening runs: %w", countError)
	}

	runs := []entities.SymbolScreeningRun{}
	listed := matching.Session(&gorm.Session{}).Order("id DESC").Limit(query.Limit).Find(&runs)
	if listed.Error != nil {
		return nil, 0, fmt.Errorf("list symbol screening runs: %w", listed.Error)
	}

	return runs, totalCount, nil
}

func (runRepository *SymbolScreeningRunRepository) CountActiveByOwner(
	executionContext context.Context, ownerID uint,
) (int, error) {
	activeCount := int64(0)
	counted := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.IN{Column: "status", Values: symbolScreeningActiveStatuses}).
		Count(&activeCount)
	if counted.Error != nil {
		return 0, fmt.Errorf("count active symbol screening runs: %w", counted.Error)
	}

	return int(activeCount), nil
}

func (runRepository *SymbolScreeningRunRepository) CountActiveByProfile(
	executionContext context.Context, profileID uint,
) (int, error) {
	activeCount := int64(0)
	counted := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "profile_id", Value: profileID}).
		Where(clause.IN{Column: "status", Values: symbolScreeningActiveStatuses}).
		Count(&activeCount)
	if counted.Error != nil {
		return 0, fmt.Errorf("count symbol screening runs using a profile: %w", counted.Error)
	}

	return int(activeCount), nil
}

func (runRepository *SymbolScreeningRunRepository) LockOldestQueued(
	executionContext context.Context,
) (entities.SymbolScreeningRun, bool, error) {
	queued := entities.SymbolScreeningRun{}
	picked := runRepository.database.within(executionContext).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked}).
		Where(clause.Eq{Column: "status", Value: string(vo.SymbolScreeningRunQueued)}).
		Order("id ASC").
		First(&queued)
	if errors.Is(picked.Error, gorm.ErrRecordNotFound) {
		return entities.SymbolScreeningRun{}, false, nil
	}
	if picked.Error != nil {
		return entities.SymbolScreeningRun{}, false, fmt.Errorf("lock queued symbol screening run: %w", picked.Error)
	}

	return queued, true, nil
}

func (runRepository *SymbolScreeningRunRepository) SaveClaim(
	executionContext context.Context, run entities.SymbolScreeningRun,
) error {
	claimed := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "id", Value: run.ID}).
		Select("status", "replica_name", "started_at").
		UpdateColumns(entities.SymbolScreeningRun{
			Status: run.Status, ReplicaName: run.ReplicaName, StartedAt: run.StartedAt,
		})
	if claimed.Error != nil {
		return fmt.Errorf("claim queued symbol screening run: %w", claimed.Error)
	}

	return nil
}

func (runRepository *SymbolScreeningRunRepository) UpdateIfRunning(
	executionContext context.Context, run entities.SymbolScreeningRun,
) (bool, error) {
	updated := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "id", Value: run.ID}).
		Where(clause.Eq{Column: "status", Value: string(vo.SymbolScreeningRunRunning)}).
		Where(clause.Eq{Column: "replica_name", Value: run.ReplicaName}).
		Select("status", "processed_candidate_count", "failure_reason", "contribution_failure_reason", "finished_at").
		Updates(map[string]any{
			"status":                      run.Status,
			"processed_candidate_count":   run.ProcessedCandidateCount,
			"failure_reason":              run.FailureReason,
			"contribution_failure_reason": run.ContributionFailureReason,
			"finished_at":                 run.FinishedAt,
		})
	if updated.Error != nil {
		return false, fmt.Errorf("update running symbol screening run: %w", updated.Error)
	}

	return updated.RowsAffected == 1, nil
}

func (runRepository *SymbolScreeningRunRepository) CancelIfActive(
	executionContext context.Context, run entities.SymbolScreeningRun,
) (bool, error) {
	cancelled := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "id", Value: run.ID}).
		Where(clause.Eq{Column: "owner_id", Value: run.OwnerID}).
		Where(clause.IN{Column: "status", Values: symbolScreeningActiveStatuses}).
		Select("status", "finished_at").
		UpdateColumns(entities.SymbolScreeningRun{Status: run.Status, FinishedAt: run.FinishedAt})
	if cancelled.Error != nil {
		return false, fmt.Errorf("cancel symbol screening run: %w", cancelled.Error)
	}

	return cancelled.RowsAffected == 1, nil
}

// FailRunningOutside answers which runs it failed, so their unfinished candidates can be cancelled too.
func (runRepository *SymbolScreeningRunRepository) FailRunningOutside(
	executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
) ([]uint, error) {
	liveReplicaValues := make([]any, 0, len(liveReplicaNames))
	for _, liveReplicaName := range liveReplicaNames {
		liveReplicaValues = append(liveReplicaValues, liveReplicaName)
	}
	abandonedIDs := []uint{}
	found := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.Eq{Column: "status", Value: string(vo.SymbolScreeningRunRunning)}).
		Where(clause.Not(clause.IN{Column: "replica_name", Values: liveReplicaValues})).
		Pluck("id", &abandonedIDs)
	if found.Error != nil {
		return nil, fmt.Errorf("find abandoned symbol screening runs: %w", found.Error)
	}
	if len(abandonedIDs) == 0 {
		return abandonedIDs, nil
	}

	abandonedValues := make([]any, 0, len(abandonedIDs))
	for _, abandonedID := range abandonedIDs {
		abandonedValues = append(abandonedValues, abandonedID)
	}
	finishedAtUtc := finishedAt.UTC()
	swept := runRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningRun{}).
		Where(clause.IN{Column: "id", Values: abandonedValues}).
		Where(clause.Eq{Column: "status", Value: string(vo.SymbolScreeningRunRunning)}).
		Select("status", "failure_reason", "finished_at").
		Updates(entities.SymbolScreeningRun{
			Status: string(vo.SymbolScreeningRunFailed), FailureReason: reason, FinishedAt: &finishedAtUtc,
		})
	if swept.Error != nil {
		return nil, fmt.Errorf("fail running symbol screening runs: %w", swept.Error)
	}

	return abandonedIDs, nil
}

func (runRepository *SymbolScreeningRunRepository) DeleteFinishedBefore(
	executionContext context.Context, cutoff time.Time,
) (int, error) {
	deleted := runRepository.database.within(executionContext).
		Where(clause.Lt{Column: "finished_at", Value: cutoff.UTC()}).
		Delete(&entities.SymbolScreeningRun{})
	if deleted.Error != nil {
		return 0, fmt.Errorf("delete expired symbol screening runs: %w", deleted.Error)
	}

	return int(deleted.RowsAffected), nil
}
