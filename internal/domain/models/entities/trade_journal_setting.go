package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

type TradeJournalSetting struct {
	ID           uint                `gorm:"primaryKey"`
	UserID       uint                `gorm:"not null;uniqueIndex:idx_trade_journal_settings_user_id"`
	MakerFeeRate decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakerFeeRate decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// DefaultMistakeTagsSeededAt stops the default tags from coming back after the person deletes them.
	DefaultMistakeTagsSeededAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt                  time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt                  time.Time  `gorm:"type:timestamptz;not null"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (tradeJournalSetting TradeJournalSetting) TableName() string {
	return "TradeJournalSettings"
}

func (tradeJournalSetting TradeJournalSetting) ToDto() dto.TradeJournalSettingDto {
	return dto.TradeJournalSettingDto{
		MakerFeeRate: tradeJournalSetting.MakerFeeRate,
		TakerFeeRate: tradeJournalSetting.TakerFeeRate,
	}
}
