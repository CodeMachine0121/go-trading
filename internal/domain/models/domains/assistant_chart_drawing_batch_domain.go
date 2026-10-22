package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

type assistantChartDrawingRequest struct {
	kind    string
	anchors []AssistantChartDrawingAnchorDomain
	style   dto.ChartDrawingStyleWriteDto
}

// AssistantChartDrawingBatchDomain is one batch the assistant draws: every anchor must sit on a real candle of the named interval, and the whole batch lands or none of it.
type AssistantChartDrawingBatchDomain struct {
	place      ChartDrawingPlaceDomain
	provenance vo.ChartDrawingProvenanceVo
	requests   []assistantChartDrawingRequest
}

// NewAssistantChartDrawingBatchDomain refuses everything that can be judged without reading the market, so a bad batch never costs a lookup.
func NewAssistantChartDrawingBatchDomain(
	batchDto dto.AssistantChartDrawingBatchDto, limits vo.ChartDrawingLimitsVo,
) (AssistantChartDrawingBatchDomain, error) {
	place, placeError := NewChartDrawingPlaceDomain(batchDto.Place)
	if placeError != nil {
		return AssistantChartDrawingBatchDomain{}, placeError
	}

	interval, intervalError := NewAggregationIntervalDomain(batchDto.Interval)
	if intervalError != nil || batchDto.Interval == "" {
		return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
			"%w: 要說出是看哪一種彙總刻度畫的；彙總刻度只能是 1m、5m、15m、1h、4h、1d 其中之一", ErrChartDrawingValidation)
	}

	if len(batchDto.Drawings) == 0 {
		return AssistantChartDrawingBatchDomain{}, fmt.Errorf("%w: 至少要一個繪圖", ErrChartDrawingValidation)
	}
	if len(batchDto.Drawings) > limits.AssistantBatchCapacity {
		return AssistantChartDrawingBatchDomain{}, fmt.Errorf("%w: 一次最多新增 %d 個繪圖，收到 %d 個",
			ErrChartDrawingValidation, limits.AssistantBatchCapacity, len(batchDto.Drawings))
	}

	requests := make([]assistantChartDrawingRequest, 0, len(batchDto.Drawings))
	for drawingPosition, writeDto := range batchDto.Drawings {
		kind, kindError := NewChartDrawingKindDomain(writeDto.Kind)
		if kindError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, kindError)
		}
		if drawableError := kind.RequireAssistantDrawable(); drawableError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, drawableError)
		}
		if anchorCountError := kind.RequireAnchorCount(len(writeDto.Anchors)); anchorCountError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, anchorCountError)
		}
		if _, styleError := NewChartDrawingStyleDomain(kind, writeDto.Style); styleError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, styleError)
		}

		anchors := make([]AssistantChartDrawingAnchorDomain, 0, len(writeDto.Anchors))
		for anchorPosition, anchorDto := range writeDto.Anchors {
			anchor, anchorError := NewAssistantChartDrawingAnchorDomain(anchorDto, interval)
			if anchorError != nil {
				return AssistantChartDrawingBatchDomain{}, fmt.Errorf("%w: 第 %d 個繪圖第 %d 個錨點：%w",
					ErrChartDrawingValidation, drawingPosition+1, anchorPosition+1, anchorError)
			}
			anchors = append(anchors, anchor)
		}

		requests = append(requests, assistantChartDrawingRequest{
			kind: writeDto.Kind, anchors: anchors, style: writeDto.Style,
		})
	}

	// A connector drawing comes from no conversation, so its answer stays empty.
	provenance := vo.ChartDrawingProvenanceVo{Source: vo.ChartDrawingSourceAssistant}
	if batchDto.ConversationID != 0 && batchDto.TurnID != 0 {
		conversationID := batchDto.ConversationID
		turnID := batchDto.TurnID
		provenance.AssistantConversationID = &conversationID
		provenance.AssistantTurnID = &turnID
	}

	return AssistantChartDrawingBatchDomain{
		place:      place,
		provenance: provenance,
		requests:   requests,
	}, nil
}

func (assistantChartDrawingBatchDomain AssistantChartDrawingBatchDomain) Place() ChartDrawingPlaceDomain {
	return assistantChartDrawingBatchDomain.place
}

// AnchorCandleQueries asks for exactly one bucket per distinct anchor time, so the candle read is the one the anchor names.
func (assistantChartDrawingBatchDomain AssistantChartDrawingBatchDomain) AnchorCandleQueries() []dto.KCandleSeriesQueryDto {
	queriedOpenTimes := make(map[time.Time]bool)
	queries := make([]dto.KCandleSeriesQueryDto, 0)
	for _, request := range assistantChartDrawingBatchDomain.requests {
		for _, anchor := range request.anchors {
			query := anchor.CandleQuery(assistantChartDrawingBatchDomain.place.Symbol())
			if queriedOpenTimes[query.StartTime] {
				continue
			}
			queriedOpenTimes[query.StartTime] = true
			queries = append(queries, query)
		}
	}

	return queries
}

// ToDrawings pins every anchor to the candle read for it, taking the snapped price where asked; an anchor with no candle refuses the whole batch.
func (assistantChartDrawingBatchDomain AssistantChartDrawingBatchDomain) ToDrawings(
	anchorCandles []dto.ChartDrawingAnchorCandleDto,
) ([]ChartDrawingDomain, error) {
	candlesByOpenTime := make(map[time.Time]dto.ChartDrawingAnchorCandleDto, len(anchorCandles))
	for _, anchorCandle := range anchorCandles {
		candlesByOpenTime[anchorCandle.OpenTime.UTC()] = anchorCandle
	}

	drawings := make([]ChartDrawingDomain, 0, len(assistantChartDrawingBatchDomain.requests))
	for drawingPosition, request := range assistantChartDrawingBatchDomain.requests {
		anchorDtos := make([]dto.ChartDrawingAnchorDto, 0, len(request.anchors))
		for anchorPosition, anchor := range request.anchors {
			anchorDto, pinError := anchor.PinnedTo(candlesByOpenTime)
			if pinError != nil {
				return nil, fmt.Errorf("%w: 第 %d 個繪圖第 %d 個錨點：%w",
					ErrChartDrawingValidation, drawingPosition+1, anchorPosition+1, pinError)
			}
			anchorDtos = append(anchorDtos, anchorDto)
		}

		drawing, drawingError := NewChartDrawingDomain(
			request.kind, anchorDtos, request.style, assistantChartDrawingBatchDomain.provenance)
		if drawingError != nil {
			return nil, fmt.Errorf("%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, drawingError)
		}

		drawings = append(drawings, drawing)
	}

	return drawings, nil
}
