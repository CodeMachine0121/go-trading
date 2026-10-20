package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

type ChartDrawingAnchorRequest struct {
	OpenTime time.Time       `json:"openTime"`
	Price    decimal.Decimal `json:"price"`
}

// ChartDrawingRequest ignores any source sent along: who drew a drawing is recorded by the system, never by the saved board.
type ChartDrawingRequest struct {
	ID        uint                        `json:"id"`
	Kind      string                      `json:"kind"`
	Anchors   []ChartDrawingAnchorRequest `json:"anchors"`
	Color     string                      `json:"color"`
	LineWidth int                         `json:"lineWidth"`
	LineStyle string                      `json:"lineStyle"`
	Locked    *bool                       `json:"locked"`
	Visible   *bool                       `json:"visible"`
}

// ChartDrawingBoardRequest requires the revision it was based on; a pointer tells a missing one from zero, the revision of a board never saved.
type ChartDrawingBoardRequest struct {
	Revision *int64                `json:"revision" binding:"required"`
	Drawings []ChartDrawingRequest `json:"drawings"`
}

func (boardRequest ChartDrawingBoardRequest) ToReplaceDto(placeDto dto.ChartDrawingBoardPlaceDto) dto.ChartDrawingBoardReplaceDto {
	writeDtos := make([]dto.ChartDrawingWriteDto, 0, len(boardRequest.Drawings))
	for _, drawingRequest := range boardRequest.Drawings {
		anchorDtos := make([]dto.ChartDrawingAnchorDto, 0, len(drawingRequest.Anchors))
		for _, anchorRequest := range drawingRequest.Anchors {
			anchorDtos = append(anchorDtos, dto.ChartDrawingAnchorDto{
				OpenTime: anchorRequest.OpenTime, Price: anchorRequest.Price,
			})
		}

		writeDtos = append(writeDtos, dto.ChartDrawingWriteDto{
			ID:      drawingRequest.ID,
			Kind:    drawingRequest.Kind,
			Anchors: anchorDtos,
			Style: dto.ChartDrawingStyleWriteDto{
				Color:     drawingRequest.Color,
				LineWidth: drawingRequest.LineWidth,
				LineStyle: drawingRequest.LineStyle,
				Locked:    drawingRequest.Locked,
				Visible:   drawingRequest.Visible,
			},
		})
	}

	return dto.ChartDrawingBoardReplaceDto{Place: placeDto, Revision: *boardRequest.Revision, Drawings: writeDtos}
}

type AssistantChartDrawingAnchorRequest struct {
	OpenTime time.Time           `json:"openTime"`
	Price    decimal.NullDecimal `json:"price"`
	SnapTo   string              `json:"snapTo"`
}

type AssistantChartDrawingRequest struct {
	Kind      string                               `json:"kind"`
	Anchors   []AssistantChartDrawingAnchorRequest `json:"anchors"`
	Color     string                               `json:"color"`
	LineWidth int                                  `json:"lineWidth"`
	LineStyle string                               `json:"lineStyle"`
}

// AssistantChartDrawingBatchRequest names no answer: drawings through this route come from a connector.
type AssistantChartDrawingBatchRequest struct {
	Interval string                         `json:"interval"`
	Drawings []AssistantChartDrawingRequest `json:"drawings"`
}

func (batchRequest AssistantChartDrawingBatchRequest) ToBatchDto(
	placeDto dto.ChartDrawingBoardPlaceDto,
) dto.AssistantChartDrawingBatchDto {
	writeDtos := make([]dto.AssistantChartDrawingWriteDto, 0, len(batchRequest.Drawings))
	for _, drawingRequest := range batchRequest.Drawings {
		anchorDtos := make([]dto.AssistantChartDrawingAnchorWriteDto, 0, len(drawingRequest.Anchors))
		for _, anchorRequest := range drawingRequest.Anchors {
			anchorDtos = append(anchorDtos, dto.AssistantChartDrawingAnchorWriteDto{
				OpenTime: anchorRequest.OpenTime, Price: anchorRequest.Price, SnapTo: anchorRequest.SnapTo,
			})
		}

		writeDtos = append(writeDtos, dto.AssistantChartDrawingWriteDto{
			Kind:    drawingRequest.Kind,
			Anchors: anchorDtos,
			Style: dto.ChartDrawingStyleWriteDto{
				Color: drawingRequest.Color, LineWidth: drawingRequest.LineWidth, LineStyle: drawingRequest.LineStyle,
			},
		})
	}

	return dto.AssistantChartDrawingBatchDto{
		Place: placeDto, Interval: batchRequest.Interval, Drawings: writeDtos,
	}
}
