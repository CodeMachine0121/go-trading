package assistantqueries

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

type chartDrawingPlaceAssistantArguments struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
}

// ToPlaceDto takes the owner from the calling capability, never from the assistant, which always draws for whoever asked it.
func (placeArguments chartDrawingPlaceAssistantArguments) ToPlaceDto(ownerID uint) dto.ChartDrawingBoardPlaceDto {
	return dto.ChartDrawingBoardPlaceDto{OwnerID: ownerID, Market: placeArguments.Market, Symbol: placeArguments.Symbol}
}

type chartDrawingAnchorAssistantArgument struct {
	OpenTime time.Time           `json:"openTime"`
	Price    decimal.NullDecimal `json:"price"`
	SnapTo   string              `json:"snapTo"`
}

type chartDrawingAssistantArgument struct {
	Kind      string                                `json:"kind"`
	Anchors   []chartDrawingAnchorAssistantArgument `json:"anchors"`
	Color     string                                `json:"color"`
	LineWidth int                                   `json:"lineWidth"`
	LineStyle string                                `json:"lineStyle"`
}

type chartDrawingCreateAssistantArguments struct {
	chartDrawingPlaceAssistantArguments
	Interval string                          `json:"interval"`
	Drawings []chartDrawingAssistantArgument `json:"drawings"`
}

// ToBatchDto ties every drawing to the answer it was drawn in.
func (createArguments chartDrawingCreateAssistantArguments) ToBatchDto(
	ownerID uint, conversationID uint, turnID uint,
) dto.AssistantChartDrawingBatchDto {
	writeDtos := make([]dto.AssistantChartDrawingWriteDto, 0, len(createArguments.Drawings))
	for _, drawingArgument := range createArguments.Drawings {
		anchorDtos := make([]dto.AssistantChartDrawingAnchorWriteDto, 0, len(drawingArgument.Anchors))
		for _, anchorArgument := range drawingArgument.Anchors {
			anchorDtos = append(anchorDtos, dto.AssistantChartDrawingAnchorWriteDto{
				OpenTime: anchorArgument.OpenTime, Price: anchorArgument.Price, SnapTo: anchorArgument.SnapTo,
			})
		}

		writeDtos = append(writeDtos, dto.AssistantChartDrawingWriteDto{
			Kind:    drawingArgument.Kind,
			Anchors: anchorDtos,
			Style: dto.ChartDrawingStyleWriteDto{
				Color: drawingArgument.Color, LineWidth: drawingArgument.LineWidth, LineStyle: drawingArgument.LineStyle,
			},
		})
	}

	return dto.AssistantChartDrawingBatchDto{
		Place:          createArguments.ToPlaceDto(ownerID),
		Interval:       createArguments.Interval,
		ConversationID: conversationID,
		TurnID:         turnID,
		Drawings:       writeDtos,
	}
}

type chartDrawingRemoveAssistantArguments struct {
	chartDrawingPlaceAssistantArguments
	DrawingIDs []uint `json:"drawingIds"`
}

// chartDrawingPlaceArgumentSchema is shared by all three drawing capabilities, which all name one board.
const chartDrawingPlaceArgumentSchema = `` +
	`"market":{"type":"string","enum":["spot","contract"],"description":"spot 是現貨 K 線圖，contract 是合約 K 線圖"},` +
	`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"}`
