package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_symbol_screening_run_repository.go -destination=mocks/mock_i_symbol_screening_run_repository.go -package=mocks

// ISymbolScreeningRunRepository guards writes from a runner with the run still running under its replica, so a cancel or a sweep is never overwritten.
type ISymbolScreeningRunRepository interface {
	// Save writes a new run together with its candidates.
	Save(executionContext context.Context, run entities.SymbolScreeningRun) (entities.SymbolScreeningRun, error)
	FindOneOwned(executionContext context.Context, ownerID uint, id uint) (entities.SymbolScreeningRun, bool, error)
	FindPageByOwner(
		executionContext context.Context, ownerID uint, query dto.SymbolScreeningRunListQueryDto,
	) ([]entities.SymbolScreeningRun, int64, error)
	CountActiveByOwner(executionContext context.Context, ownerID uint) (int, error)
	CountActiveByProfile(executionContext context.Context, profileID uint) (int, error)
	// LockOldestQueued must run inside a transaction, which holds the row lock until the claim is saved.
	LockOldestQueued(executionContext context.Context) (entities.SymbolScreeningRun, bool, error)
	SaveClaim(executionContext context.Context, run entities.SymbolScreeningRun) error
	UpdateIfRunning(executionContext context.Context, run entities.SymbolScreeningRun) (bool, error)
	CancelIfActive(executionContext context.Context, run entities.SymbolScreeningRun) (bool, error)
	FailRunningOutside(
		executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
	) (int, error)
	DeleteFinishedBefore(executionContext context.Context, cutoff time.Time) (int, error)
}
