package _interface

import (
	"context"
	"time"
)

//go:generate go tool mockgen -source=i_strategy_script_adoption_repository.go -destination=mocks/mock_i_strategy_script_adoption_repository.go -package=mocks

// IStrategyScriptAdoptionRepository records which published scripts each person adopted; both writes are idempotent, and reads live on the strategy script repository.
type IStrategyScriptAdoptionRepository interface {
	// Adopt returns ErrStrategyScriptNotFound when the script is not on the marketplace.
	Adopt(executionContext context.Context, userID uint, strategyScriptID uint, adoptedAt time.Time) error
	// Abandon is a no-op for scripts never adopted.
	Abandon(executionContext context.Context, userID uint, strategyScriptID uint) error
}
