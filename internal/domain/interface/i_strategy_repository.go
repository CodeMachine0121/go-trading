package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

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
	Save(executionContext context.Context, strategy entities.Strategy) (entities.Strategy, error)
	// Update rewrites the five things a strategy remembers, leaving its identifier
	// and the time it was first saved untouched. Refuses with ErrStrategyNotFound
	// when no strategy carries that identifier, and with ErrStrategyNameConflict
	// when the new name belongs to a different strategy. Renaming a strategy to the
	// name it already has is not a conflict — the name it collides with is its own.
	Update(executionContext context.Context, strategy entities.Strategy) (entities.Strategy, error)
	// FindOne returns the strategy carrying this identifier, or ErrStrategyNotFound.
	FindOne(executionContext context.Context, id uint) (entities.Strategy, error)
	// FindAllOwnedBy returns every strategy belonging to this owner, ordered by
	// name. Owning none is an answer, not a failure.
	FindAllOwnedBy(executionContext context.Context, ownerID uint) ([]entities.Strategy, error)
	// FindAllPublished returns every strategy that is on the marketplace, newest
	// publication first, each one carrying its owner and the moment it was
	// published. Reading the publications and the strategies together is one
	// question — "what is on the marketplace" — and answering it in one place is
	// what keeps a caller from paging through identifiers to fill in the rest.
	FindAllPublished(executionContext context.Context) ([]entities.PublishedStrategy, error)
	// FindAllAdoptedBy returns every strategy this user has taken from the
	// marketplace and that is still on it, ordered by name, each carrying its owner
	// and its publication. A withdrawn strategy cannot appear: the adoption went
	// with the publication.
	FindAllAdoptedBy(executionContext context.Context, userID uint) ([]entities.PublishedStrategy, error)
	// Delete removes the strategy carrying this identifier for good, freeing its
	// name and taking its publication and everybody's adoption of it with it.
	// Refuses with ErrStrategyNotFound when there is no such strategy.
	Delete(executionContext context.Context, id uint) error
}
