package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type ContractTradeNote struct {
	ID                    uint      `gorm:"primaryKey"`
	ContractTradeRecordID uint      `gorm:"not null;index"`
	Content               string    `gorm:"type:text;not null"`
	CreatedAt             time.Time `gorm:"type:timestamptz;not null"`
}

func (contractTradeNote ContractTradeNote) TableName() string {
	return "ContractTradeNotes"
}

func (contractTradeNote ContractTradeNote) ToDto() dto.ContractTradeNoteDto {
	return dto.ContractTradeNoteDto{
		ID:        contractTradeNote.ID,
		Content:   contractTradeNote.Content,
		CreatedAt: contractTradeNote.CreatedAt.UTC(),
	}
}
