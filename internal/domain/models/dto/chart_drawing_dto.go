package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ChartDrawingDto is also the stored shape of a drawing on its board, so new fields must be optional.
type ChartDrawingDto struct {
	ID        uint                    `json:"id"`
	Kind      string                  `json:"kind"`
	Anchors   []ChartDrawingAnchorDto `json:"anchors"`
	Color     string                  `json:"color"`
	LineWidth int                     `json:"lineWidth"`
	LineStyle string                  `json:"lineStyle"`
	Locked    bool                    `json:"locked"`
	Visible   bool                    `json:"visible"`
	Source    string                  `json:"source"`
	// AssistantConversationID and AssistantTurnID name the in-app answer that drew it; both are null for hand drawings and connector drawings.
	AssistantConversationID *uint `json:"assistantConversationId"`
	AssistantTurnID         *uint `json:"assistantTurnId"`
}

// ChartDrawingAnchorDto is one point of a drawing: the open time of a candle and a price.
type ChartDrawingAnchorDto struct {
	OpenTime time.Time       `json:"openTime"`
	Price    decimal.Decimal `json:"price"`
}

// ChartDrawingStyleWriteDto leaves every field optional; anything not given takes the kind's default.
type ChartDrawingStyleWriteDto struct {
	Color     string
	LineWidth int
	LineStyle string
	Locked    *bool
	Visible   *bool
}

// ChartDrawingWriteDto is one drawing of a board saved whole; a zero or unknown identifier means newly drawn.
type ChartDrawingWriteDto struct {
	ID      uint
	Kind    string
	Anchors []ChartDrawingAnchorDto
	Style   ChartDrawingStyleWriteDto
}
