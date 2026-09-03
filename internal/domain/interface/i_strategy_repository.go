package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

//go:generate go tool mockgen -source=i_strategy_repository.go -destination=mocks/mock_i_strategy_repository.go -package=mocks

// IStrategyRepository stores and retrieves saved strategies.
//
// Two questions are answered here rather than by whoever calls: whether a name is
// already taken, and whether the strategy named by an identifier exists. Both are
// facts about what is stored, and a caller that checked them first would be reading
// state it then acts on separately — which two callers arriving at once turn into
// two strategies of the same name.
type IStrategyRepository interface {
	// Save stores a new strategy and returns it as stored, identifier and times
	// filled in. A name another strategy already holds is refused with
	// ErrStrategyNameConflict.
	Save(strategy entities.Strategy) (entities.Strategy, error)
	// Update rewrites the five things a strategy remembers, leaving its identifier
	// and the time it was first saved untouched. Refuses with ErrStrategyNotFound
	// when no strategy carries that identifier, and with ErrStrategyNameConflict
	// when the new name belongs to a different strategy. Renaming a strategy to the
	// name it already has is not a conflict — the name it collides with is its own.
	Update(strategy entities.Strategy) (entities.Strategy, error)
	// FindOne returns the strategy carrying this identifier, or ErrStrategyNotFound.
	FindOne(id uint) (entities.Strategy, error)
	// FindAll returns every saved strategy, ordered by name.
	FindAll() ([]entities.Strategy, error)
	// Delete removes the strategy carrying this identifier for good, freeing its
	// name. Refuses with ErrStrategyNotFound when there is no such strategy.
	Delete(id uint) error
}
