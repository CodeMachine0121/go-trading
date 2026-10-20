package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// AssistantChartDrawingBatchDto is one batch the assistant draws; a zero conversation or turn means it came through a connector.
type AssistantChartDrawingBatchDto struct {
	Place          ChartDrawingBoardPlaceDto
	Interval       string
	ConversationID uint
	TurnID         uint
	Drawings       []AssistantChartDrawingWriteDto
}

type AssistantChartDrawingWriteDto struct {
	Kind    string
	Anchors []AssistantChartDrawingAnchorWriteDto
	Style   ChartDrawingStyleWriteDto
}

// AssistantChartDrawingAnchorWriteDto takes exactly one of a price and a snap to the anchored candle.
type AssistantChartDrawingAnchorWriteDto struct {
	OpenTime time.Time
	Price    decimal.NullDecimal
	SnapTo   string
}

type AssistantChartDrawingAdditionDto struct {
	Revision      int64             `json:"revision"`
	AddedDrawings []ChartDrawingDto `json:"addedDrawings"`
}

// AssistantChartDrawingRemovalDto with no identifiers removes every drawing the assistant drew on the board.
type AssistantChartDrawingRemovalDto struct {
	Place      ChartDrawingBoardPlaceDto
	DrawingIDs []uint
}

type AssistantChartDrawingRemovalResultDto struct {
	Revision          int64  `json:"revision"`
	RemovedCount      int    `json:"removedCount"`
	RemovedDrawingIDs []uint `json:"removedDrawingIds"`
}
