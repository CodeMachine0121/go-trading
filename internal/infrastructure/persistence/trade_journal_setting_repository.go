package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradeJournalSettingRepository struct {
	database *gorm.DB
}

func NewTradeJournalSettingRepository(database *gorm.DB) *TradeJournalSettingRepository {
	return &TradeJournalSettingRepository{database: database}
}

func (tradeJournalSettingRepository *TradeJournalSettingRepository) FindOneByUser(
	executionContext context.Context, userID uint,
) (entities.TradeJournalSetting, bool, error) {
	setting := entities.TradeJournalSetting{}

	result := tradeJournalSettingRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "user_id", Value: userID}).
		First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.TradeJournalSetting{}, false, nil
	}
	if result.Error != nil {
		return entities.TradeJournalSetting{}, false, fmt.Errorf("find trade journal setting: %w", result.Error)
	}

	return setting, true, nil
}

// SaveFeeRates relies on the unique user index so concurrent writes cannot create two rows.
func (tradeJournalSettingRepository *TradeJournalSettingRepository) SaveFeeRates(
	executionContext context.Context, setting entities.TradeJournalSetting,
) (entities.TradeJournalSetting, error) {
	row := entities.TradeJournalSetting{
		UserID:       setting.UserID,
		MakerFeeRate: setting.MakerFeeRate,
		TakerFeeRate: setting.TakerFeeRate,
	}

	result := tradeJournalSettingRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"maker_fee_rate", "taker_fee_rate", "updated_at"}),
		}).
		Create(&row)
	if result.Error != nil {
		return entities.TradeJournalSetting{}, fmt.Errorf("save trade journal fee rates: %w", result.Error)
	}

	storedSetting, _, findError := tradeJournalSettingRepository.FindOneByUser(executionContext, setting.UserID)

	return storedSetting, findError
}

func (tradeJournalSettingRepository *TradeJournalSettingRepository) MarkDefaultMistakeTagsSeeded(
	executionContext context.Context, userID uint, seededAt time.Time,
) error {
	seededAtUtc := seededAt.UTC()
	row := entities.TradeJournalSetting{UserID: userID, DefaultMistakeTagsSeededAt: &seededAtUtc}

	result := tradeJournalSettingRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"default_mistake_tags_seeded_at", "updated_at"}),
		}).
		Create(&row)
	if result.Error != nil {
		return fmt.Errorf("mark default mistake tags seeded: %w", result.Error)
	}

	return nil
}
