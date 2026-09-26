package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_script_repository.go -destination=mocks/mock_i_strategy_script_repository.go -package=mocks

// IStrategyScriptRepository enforces name uniqueness and existence at write time so concurrent callers cannot race.
type IStrategyScriptRepository interface {
	// Save returns ErrStrategyScriptNameConflict when another script holds the name.
	Save(executionContext context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error)
	// Update keeps identifier and creation time; returns ErrStrategyScriptNotFound or ErrStrategyScriptNameConflict (renaming to its own name is fine).
	Update(executionContext context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error)
	// FindOne returns ErrStrategyScriptNotFound when absent.
	FindOne(executionContext context.Context, id uint) (entities.StrategyScript, error)
	// FindAllOwnedBy is ordered by name.
	FindAllOwnedBy(executionContext context.Context, ownerID uint) ([]entities.StrategyScript, error)
	// FindAllPublished returns published scripts newest publication first, with owner and publication moment.
	FindAllPublished(executionContext context.Context) ([]entities.PublishedStrategyScript, error)
	// Delete also removes its publication; marketplace copies of it are separate scripts and stay. Returns ErrStrategyScriptNotFound when absent.
	Delete(executionContext context.Context, id uint) error
}
