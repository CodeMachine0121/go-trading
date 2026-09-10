package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TelegramDeliveryRepository stores where each person wants this system to speak to
// them, in PostgreSQL.
type TelegramDeliveryRepository struct {
	database *gorm.DB
}

func NewTelegramDeliveryRepository(database *gorm.DB) *TelegramDeliveryRepository {
	return &TelegramDeliveryRepository{database: database}
}

// FindOneByUser returns this person's setting.
func (telegramDeliveryRepository *TelegramDeliveryRepository) FindOneByUser(
	executionContext context.Context, userID uint,
) (entities.TelegramDelivery, error) {
	delivery := entities.TelegramDelivery{}

	// The condition is spelled out rather than given as a struct, because GORM
	// drops zero-valued struct fields — so an identifier of nobody would become no
	// condition at all, and this would hand back whichever setting happens to be
	// first in the table, along with somebody else's token.
	result := telegramDeliveryRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "user_id", Value: userID}).
		First(&delivery)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured
	}
	if result.Error != nil {
		return entities.TelegramDelivery{}, fmt.Errorf("find telegram delivery: %w", result.Error)
	}

	return delivery, nil
}

// Upsert stores this setting, replacing whatever this person had before.
//
// The unique index on the person is what decides, rather than a look followed by a
// write: two settings arriving at once would both find nothing there and both
// insert, and only the index stops that becoming two rows. Naming the index in the
// conflict clause is how the write says which collision it means to absorb, so that
// any other broken constraint still surfaces as the failure it is.
func (telegramDeliveryRepository *TelegramDeliveryRepository) Upsert(
	executionContext context.Context, delivery entities.TelegramDelivery,
) (entities.TelegramDelivery, error) {
	result := telegramDeliveryRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns(
				[]string{"sealed_bot_token", "bot_token_tail", "chat_id", "updated_at"}),
		}).
		Create(&delivery)
	if result.Error != nil {
		return entities.TelegramDelivery{}, fmt.Errorf("save telegram delivery: %w", result.Error)
	}

	return delivery, nil
}

// DeleteByUser removes this person's setting.
//
// Deleting nothing is success. What was asked for is that this system stop being
// able to speak to them, and with no setting it already cannot — reporting a failure
// would only have the caller pressing the button again.
func (telegramDeliveryRepository *TelegramDeliveryRepository) DeleteByUser(
	executionContext context.Context, userID uint,
) error {
	result := telegramDeliveryRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "user_id", Value: userID}).
		Delete(&entities.TelegramDelivery{})
	if result.Error != nil {
		return fmt.Errorf("delete telegram delivery: %w", result.Error)
	}

	return nil
}
