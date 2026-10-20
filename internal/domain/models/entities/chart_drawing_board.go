package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ChartDrawingBoard keeps a board's drawings whole in one row, so every write is a single conditional update on Revision.
type ChartDrawingBoard struct {
	ID      uint   `gorm:"primaryKey"`
	OwnerID uint   `gorm:"not null;uniqueIndex:idx_chart_drawing_boards_owner_market_symbol,priority:1"`
	Market  string `gorm:"size:16;not null;uniqueIndex:idx_chart_drawing_boards_owner_market_symbol,priority:2"`
	Symbol  string `gorm:"size:64;not null;uniqueIndex:idx_chart_drawing_boards_owner_market_symbol,priority:3"`
	// Revision moves on with every write; a board never stored has revision zero.
	Revision int64 `gorm:"not null"`
	// LastDrawingID numbers drawings within the board and never goes back, so a removed drawing's number is never reused.
	LastDrawingID uint                  `gorm:"not null"`
	Drawings      []dto.ChartDrawingDto `gorm:"serializer:json;type:jsonb;not null"`
	UpdatedAt     time.Time             `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
}

func (chartDrawingBoard ChartDrawingBoard) TableName() string {
	return "ChartDrawingBoards"
}
