package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type SpotTradeNote struct {
	ID                uint      `gorm:"primaryKey"`
	SpotTradeRecordID uint      `gorm:"not null;index"`
	Content           string    `gorm:"type:text;not null"`
	CreatedAt         time.Time `gorm:"type:timestamptz;not null"`
}

func (spotTradeNote SpotTradeNote) TableName() string {
	return "SpotTradeNotes"
}

func (spotTradeNote SpotTradeNote) ToDto() dto.SpotTradeNoteDto {
	return dto.SpotTradeNoteDto{
		ID:        spotTradeNote.ID,
		Content:   spotTradeNote.Content,
		CreatedAt: spotTradeNote.CreatedAt.UTC(),
	}
}
