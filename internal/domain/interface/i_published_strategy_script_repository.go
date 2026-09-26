package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_published_strategy_script_repository.go -destination=mocks/mock_i_published_strategy_script_repository.go -package=mocks

// IPublishedStrategyScriptRepository records marketplace publications; publish and withdraw are idempotent end states.
type IPublishedStrategyScriptRepository interface {
	// Publish keeps the original publication moment when the script is already published.
	Publish(executionContext context.Context, strategyScriptID uint, publishedAt time.Time) error
	// Withdraw leaves adopted copies alone; withdrawing an unpublished script is a no-op.
	Withdraw(executionContext context.Context, strategyScriptID uint) error
	// FindOne returns ErrStrategyScriptNotPublished when the script is not on the marketplace.
	FindOne(executionContext context.Context, strategyScriptID uint) (entities.PublishedStrategyScript, error)
}
