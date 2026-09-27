package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trade_tag_repository.go -destination=mocks/mock_i_trade_tag_repository.go -package=mocks

// ITradeTagRepository leaves ownership checks to the domain; lookups by identifier return any owner's tag.
type ITradeTagRepository interface {
	// FindAllByOwner orders by kind, then name.
	FindAllByOwner(executionContext context.Context, ownerID uint) ([]entities.TradeTag, error)
	FindOne(executionContext context.Context, id uint) (entities.TradeTag, error)
	FindByIDs(executionContext context.Context, ids []uint) ([]entities.TradeTag, error)
	// Create reports a name already used by the same owner for the same kind as a name conflict.
	Create(executionContext context.Context, tag entities.TradeTag) (entities.TradeTag, error)
	// CreateIfAbsent skips names the owner already has, so seeding twice is harmless.
	CreateIfAbsent(executionContext context.Context, tags []entities.TradeTag) error
	Rename(executionContext context.Context, id uint, name string) (entities.TradeTag, error)
	Delete(executionContext context.Context, id uint) error
}
