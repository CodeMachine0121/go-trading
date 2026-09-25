package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_telegram_delivery_repository.go -destination=mocks/mock_i_telegram_delivery_repository.go -package=mocks

// ITelegramDeliveryRepository keys every method by user, enforcing at most one setting per person.
type ITelegramDeliveryRepository interface {
	// FindOneByUser returns ErrTelegramDeliveryNotConfigured when none is set.
	FindOneByUser(executionContext context.Context, userID uint) (entities.TelegramDelivery, error)
	// Upsert replaces any existing setting atomically so concurrent writes rely on the unique index rather than a read-then-write.
	Upsert(
		executionContext context.Context, delivery entities.TelegramDelivery,
	) (entities.TelegramDelivery, error)
	// DeleteByUser is a no-op when nothing is set.
	DeleteByUser(executionContext context.Context, userID uint) error
}
