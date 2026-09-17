package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_script_repository.go -destination=mocks/mock_i_strategy_script_repository.go -package=mocks

// IStrategyScriptRepository stores and retrieves saved strategy scripts.
//
// Two questions are answered here rather than by whoever calls: whether a name is
// already taken, and whether the strategy script named by an identifier exists. Both are
// facts about what is stored, and a caller that checked them first would be reading
// state it then acts on separately — which two callers arriving at once turn into
// two strategy scripts of the same name.
type IStrategyScriptRepository interface {
	// Save stores a new strategy script and returns it as stored, identifier and times
	// filled in. A name another strategy script already holds is refused with
	// ErrStrategyScriptNameConflict.
	Save(executionContext context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error)
	// Update rewrites the five things a strategy script remembers, leaving its identifier
	// and the time it was first saved untouched. Refuses with ErrStrategyScriptNotFound
	// when no strategy script carries that identifier, and with ErrStrategyScriptNameConflict
	// when the new name belongs to a different strategy script. Renaming a strategy script to the
	// name it already has is not a conflict — the name it collides with is its own.
	Update(executionContext context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error)
	// FindOne returns the strategy script carrying this identifier, or ErrStrategyScriptNotFound.
	FindOne(executionContext context.Context, id uint) (entities.StrategyScript, error)
	// FindAllOwnedBy returns every strategy script belonging to this owner, ordered by
	// name. Owning none is an answer, not a failure.
	FindAllOwnedBy(executionContext context.Context, ownerID uint) ([]entities.StrategyScript, error)
	// FindAllPublished returns every strategy script that is on the marketplace, newest
	// publication first, each one carrying its owner and the moment it was
	// published. Reading the publications and the strategy scripts together is one
	// question — "what is on the marketplace" — and answering it in one place is
	// what keeps a caller from paging through identifiers to fill in the rest.
	FindAllPublished(executionContext context.Context) ([]entities.PublishedStrategyScript, error)
	// FindAllAdoptedBy returns every strategy script this user has taken from the
	// marketplace and that is still on it, ordered by name, each carrying its owner
	// and its publication. A withdrawn strategy script cannot appear: the adoption went
	// with the publication.
	FindAllAdoptedBy(executionContext context.Context, userID uint) ([]entities.PublishedStrategyScript, error)
	// Delete removes the strategy script carrying this identifier for good, freeing its
	// name and taking its publication and everybody's adoption of it with it.
	// Refuses with ErrStrategyScriptNotFound when there is no such strategy script.
	Delete(executionContext context.Context, id uint) error
}
