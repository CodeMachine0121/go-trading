package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_telegram_delivery_repository.go -destination=mocks/mock_i_telegram_delivery_repository.go -package=mocks

// ITelegramDeliveryRepository stores where each person wants this system to speak to
// them.
//
// Every method takes the person, and none of them takes a setting identifier. That
// is what makes "one person, at most one setting" a fact about the interface rather
// than a convention somebody upholds: there is no way to ask for a second one, and
// no way to name somebody else's.
type ITelegramDeliveryRepository interface {
	// FindOneByUser returns this person's setting, or
	// ErrTelegramDeliveryNotConfigured.
	FindOneByUser(executionContext context.Context, userID uint) (entities.TelegramDelivery, error)
	// Upsert stores this setting, replacing whatever this person had before, and
	// returns it as stored.
	//
	// Replacing is part of the write rather than something a caller arranges by
	// looking first. Looking and then writing are two moments, and two settings
	// arriving at once both look and find nothing — after which only the unique
	// index stops there being two. Saying "upsert" is how that index gets to be
	// the thing that decides, instead of the thing that occasionally surprises
	// somebody.
	Upsert(
		executionContext context.Context, delivery entities.TelegramDelivery,
	) (entities.TelegramDelivery, error)
	// DeleteByUser removes this person's setting.
	//
	// Removing one that is not there is not a failure: what was asked for is that
	// this system stop being able to speak to them, and it already cannot. An
	// error would have the caller pressing the button again to reach a state it is
	// already in.
	DeleteByUser(executionContext context.Context, userID uint) error
}
