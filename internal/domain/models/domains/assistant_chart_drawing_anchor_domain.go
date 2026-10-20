package domains

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// AssistantChartDrawingAnchorDomain is one anchor the assistant asked for: an open time on the interval's grid and exactly one of a price and a snap to that candle.
type AssistantChartDrawingAnchorDomain struct {
	openTime time.Time
	interval AggregationIntervalDomain
	price    decimal.NullDecimal
	snapTo   vo.ChartDrawingAnchorSnapVo
}

func NewAssistantChartDrawingAnchorDomain(
	anchorDto dto.AssistantChartDrawingAnchorWriteDto, interval AggregationIntervalDomain,
) (AssistantChartDrawingAnchorDomain, error) {
	openTime := anchorDto.OpenTime.UTC()
	if openTime.IsZero() || interval.BucketStart(openTime) != openTime {
		return AssistantChartDrawingAnchorDomain{}, fmt.Errorf(
			"%s 不是 %s K 線的起始時間", openTime.Format(time.RFC3339), interval.Value())
	}

	snapTo := vo.ChartDrawingAnchorSnapVo(anchorDto.SnapTo)
	hasSnap := snapTo != ""
	if hasSnap == anchorDto.Price.Valid {
		return AssistantChartDrawingAnchorDomain{}, errors.New("價格（price）與吸附（snapTo）要恰好給一種")
	}
	if hasSnap && snapTo != vo.ChartDrawingAnchorSnapOpen && snapTo != vo.ChartDrawingAnchorSnapHigh &&
		snapTo != vo.ChartDrawingAnchorSnapLow && snapTo != vo.ChartDrawingAnchorSnapClose {
		return AssistantChartDrawingAnchorDomain{}, fmt.Errorf(
			"吸附只能是 open、high、low、close 其中之一，收到的是「%s」", anchorDto.SnapTo)
	}
	if anchorDto.Price.Valid && !anchorDto.Price.Decimal.IsPositive() {
		return AssistantChartDrawingAnchorDomain{}, errors.New("錨點價格必須大於零")
	}

	return AssistantChartDrawingAnchorDomain{
		openTime: openTime, interval: interval, price: anchorDto.Price, snapTo: snapTo,
	}, nil
}

// CandleQuery reads exactly the one bucket the anchor names.
func (anchorDomain AssistantChartDrawingAnchorDomain) CandleQuery(symbol string) dto.KCandleSeriesQueryDto {
	return dto.KCandleSeriesQueryDto{
		Symbol:    symbol,
		StartTime: anchorDomain.openTime,
		EndTime:   anchorDomain.interval.BucketEnd(anchorDomain.openTime),
		Interval:  string(anchorDomain.interval.Value()),
	}
}

// PinnedTo finds the anchor's candle among those read and takes its snapped price, or the given one; no such candle refuses the anchor.
func (anchorDomain AssistantChartDrawingAnchorDomain) PinnedTo(
	candlesByOpenTime map[time.Time]dto.ChartDrawingAnchorCandleDto,
) (dto.ChartDrawingAnchorDto, error) {
	anchorCandle, candleExists := candlesByOpenTime[anchorDomain.openTime]
	if !candleExists {
		return dto.ChartDrawingAnchorDto{}, fmt.Errorf("%s 沒有 %s K 線，錨點要釘在一根真的 K 線上",
			anchorDomain.openTime.Format(time.RFC3339), anchorDomain.interval.Value())
	}

	price := map[vo.ChartDrawingAnchorSnapVo]decimal.Decimal{
		vo.ChartDrawingAnchorSnapOpen:  anchorCandle.Open,
		vo.ChartDrawingAnchorSnapHigh:  anchorCandle.High,
		vo.ChartDrawingAnchorSnapLow:   anchorCandle.Low,
		vo.ChartDrawingAnchorSnapClose: anchorCandle.Close,
		"":                             anchorDomain.price.Decimal,
	}[anchorDomain.snapTo]

	return dto.ChartDrawingAnchorDto{OpenTime: anchorDomain.openTime, Price: price}, nil
}
