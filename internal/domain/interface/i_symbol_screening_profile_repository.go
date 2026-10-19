package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_symbol_screening_profile_repository.go -destination=mocks/mock_i_symbol_screening_profile_repository.go -package=mocks

type ISymbolScreeningProfileRepository interface {
	// Save creates a profile without an ID and rewrites the one with it.
	Save(executionContext context.Context, profile entities.SymbolScreeningProfile) (entities.SymbolScreeningProfile, error)
	FindOneOwned(executionContext context.Context, ownerID uint, id uint) (entities.SymbolScreeningProfile, bool, error)
	FindAllByOwner(executionContext context.Context, ownerID uint) ([]entities.SymbolScreeningProfile, error)
	// ExistsName ignores the profile being rewritten, so keeping one's own name is not a clash.
	ExistsName(executionContext context.Context, ownerID uint, name string, exceptID uint) (bool, error)
	DeleteOwned(executionContext context.Context, ownerID uint, id uint) error
}
