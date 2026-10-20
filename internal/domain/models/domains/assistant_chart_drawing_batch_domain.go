package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// assistantChartDrawingAnchorRequest is one anchor the assistant asked for, already checked to name exactly one of a price and a snap.
type assistantChartDrawingAnchorRequest struct {
	openTime time.Time
	price    decimal.NullDecimal
	snapTo   vo.ChartDrawingAnchorSnapVo
}

type assistantChartDrawingRequest struct {
	kind    string
	anchors []assistantChartDrawingAnchorRequest
	style   dto.ChartDrawingStyleWriteDto
}

// AssistantChartDrawingBatchDomain is one batch the assistant draws: every anchor must sit on a real candle of the named interval, and the whole batch lands or none of it.
type AssistantChartDrawingBatchDomain struct {
	place      ChartDrawingPlaceDomain
	interval   AggregationIntervalDomain
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
		if anchorCountError := kind.RequireAnchorCount(len(writeDto.Anchors)); anchorCountError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, anchorCountError)
		}
		if _, styleError := NewChartDrawingStyleDomain(kind, writeDto.Style); styleError != nil {
			return AssistantChartDrawingBatchDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, drawingPosition+1, styleError)
		}

		anchors := make([]assistantChartDrawingAnchorRequest, 0, len(writeDto.Anchors))
		for anchorPosition, anchorDto := range writeDto.Anchors {
			anchorError := func() error {
				openTime := anchorDto.OpenTime.UTC()
				if openTime.IsZero() || interval.BucketStart(openTime) != openTime {
					return fmt.Errorf("%s 不是 %s K 線的起始時間",
						openTime.Format(time.RFC3339), interval.Value())
				}

				snapTo := vo.ChartDrawingAnchorSnapVo(anchorDto.SnapTo)
				hasSnap := anchorDto.SnapTo != ""
				if hasSnap == anchorDto.Price.Valid {
					return fmt.Errorf("價格（price）與吸附（snapTo）要恰好給一種")
				}
				if hasSnap && snapTo != vo.ChartDrawingAnchorSnapOpen && snapTo != vo.ChartDrawingAnchorSnapHigh &&
					snapTo != vo.ChartDrawingAnchorSnapLow && snapTo != vo.ChartDrawingAnchorSnapClose {
					return fmt.Errorf("吸附只能是 open、high、low、close 其中之一，收到的是「%s」", anchorDto.SnapTo)
				}
				if anchorDto.Price.Valid && !anchorDto.Price.Decimal.IsPositive() {
					return fmt.Errorf("錨點價格必須大於零")
				}

				anchors = append(anchors, assistantChartDrawingAnchorRequest{
					openTime: openTime, price: anchorDto.Price, snapTo: snapTo,
				})

				return nil
			}()
			if anchorError != nil {
				return AssistantChartDrawingBatchDomain{}, fmt.Errorf("%w: 第 %d 個繪圖第 %d 個錨點：%w",
					ErrChartDrawingValidation, drawingPosition+1, anchorPosition+1, anchorError)
			}
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
		interval:   interval,
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
			if queriedOpenTimes[anchor.openTime] {
				continue
			}
			queriedOpenTimes[anchor.openTime] = true

			queries = append(queries, dto.KCandleSeriesQueryDto{
				Symbol:    assistantChartDrawingBatchDomain.place.Symbol(),
				StartTime: anchor.openTime,
				EndTime:   assistantChartDrawingBatchDomain.interval.BucketEnd(anchor.openTime),
				Interval:  string(assistantChartDrawingBatchDomain.interval.Value()),
			})
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
			anchorCandle, candleExists := candlesByOpenTime[anchor.openTime]
			if !candleExists {
				return nil, fmt.Errorf("%w: 第 %d 個繪圖第 %d 個錨點：%s 沒有 %s K 線，錨點要釘在一根真的 K 線上",
					ErrChartDrawingValidation, drawingPosition+1, anchorPosition+1,
					anchor.openTime.Format(time.RFC3339), assistantChartDrawingBatchDomain.interval.Value())
			}

			price := map[vo.ChartDrawingAnchorSnapVo]decimal.Decimal{
				vo.ChartDrawingAnchorSnapOpen:  anchorCandle.Open,
				vo.ChartDrawingAnchorSnapHigh:  anchorCandle.High,
				vo.ChartDrawingAnchorSnapLow:   anchorCandle.Low,
				vo.ChartDrawingAnchorSnapClose: anchorCandle.Close,
				"":                             anchor.price.Decimal,
			}[anchor.snapTo]

			anchorDtos = append(anchorDtos, dto.ChartDrawingAnchorDto{OpenTime: anchor.openTime, Price: price})
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
