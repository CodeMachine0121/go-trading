package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_published_strategy_repository.go -destination=mocks/mock_i_published_strategy_repository.go -package=mocks

// IPublishedStrategyRepository records which strategies are on the marketplace.
//
// Publishing and withdrawing are both stated as the state to end in rather than as
// an event, so saying it twice says the same thing as saying it once. Two people
// pressing publish at the same moment leave one row, and pressing withdraw on
// something already withdrawn is not a failure — it is somebody agreeing with the
// world.
type IPublishedStrategyRepository interface {
	// Publish puts this strategy on the marketplace as of this moment, or leaves it
	// where it already is. An already-published strategy keeps the moment it first
	// got there: it has been out since then, and pressing the button again does not
	// change when that started.
	Publish(executionContext context.Context, strategyID uint, publishedAt time.Time) error
	// Withdraw takes this strategy off the marketplace, clearing every adoption of
	// it on the way out. Withdrawing one that is not on the marketplace does
	// nothing and is not a failure.
	Withdraw(executionContext context.Context, strategyID uint) error
	// FindOne returns this strategy's place on the marketplace, or
	// ErrStrategyNotPublished when it has none. It answers the third gate.
	FindOne(executionContext context.Context, strategyID uint) (entities.PublishedStrategy, error)
}
