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

type TelegramDeliveryRepository struct {
	database *gorm.DB
}

func NewTelegramDeliveryRepository(database *gorm.DB) *TelegramDeliveryRepository {
	return &TelegramDeliveryRepository{database: database}
}

func (telegramDeliveryRepository *TelegramDeliveryRepository) FindOneByUser(
	executionContext context.Context, userID uint,
) (entities.TelegramDelivery, error) {
	delivery := entities.TelegramDelivery{}

	// A string condition is used because GORM drops zero-valued struct fields, which would match any row.
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

// Upsert relies on the unique user index (named in the conflict clause) so concurrent writes cannot create two rows and other constraint violations still surface.
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

// DeleteByUser treats deleting nothing as success.
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
