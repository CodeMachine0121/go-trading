package _interface

import (
	"context"
	"time"
)

//go:generate go tool mockgen -source=i_strategy_adoption_repository.go -destination=mocks/mock_i_strategy_adoption_repository.go -package=mocks

// IStrategyAdoptionRepository records which published strategies each person keeps
// on their own shelf.
//
// Like publishing, both writes state the state to end in: adopting something twice
// leaves one row, and dropping something never adopted is somebody agreeing with
// the world rather than a failure.
//
// There is no read here. What a person's shelf holds is answered by the strategy
// store, which returns the strategies themselves — a list of identifiers would only
// send the caller off to fetch them one at a time.
type IStrategyAdoptionRepository interface {
	// Adopt puts this published strategy on this person's shelf as of this moment,
	// or leaves it where it already is. Refuses with ErrStrategyNotFound when the
	// strategy is not on the marketplace — there is nothing there to take.
	Adopt(executionContext context.Context, userID uint, strategyID uint, adoptedAt time.Time) error
	// Abandon takes it off their shelf. Doing so to something they never took is not
	// a failure.
	Abandon(executionContext context.Context, userID uint, strategyID uint) error
}
