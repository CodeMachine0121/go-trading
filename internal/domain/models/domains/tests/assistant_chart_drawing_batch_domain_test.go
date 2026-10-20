package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var assistantAnchorMoment = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

func snappedAnchor(openTime time.Time, snapTo string) dto.AssistantChartDrawingAnchorWriteDto {
	return dto.AssistantChartDrawingAnchorWriteDto{OpenTime: openTime, SnapTo: snapTo}
}

func pricedAnchor(openTime time.Time, priceText string) dto.AssistantChartDrawingAnchorWriteDto {
	return dto.AssistantChartDrawingAnchorWriteDto{
		OpenTime: openTime, Price: decimal.NewNullDecimal(decimal.RequireFromString(priceText)),
	}
}

func assistantBatch(interval string, drawings ...dto.AssistantChartDrawingWriteDto) dto.AssistantChartDrawingBatchDto {
	return dto.AssistantChartDrawingBatchDto{
		Place:    dto.ChartDrawingBoardPlaceDto{OwnerID: 7, Market: "spot", Symbol: "BTCUSDT"},
		Interval: interval, ConversationID: 12, TurnID: 34, Drawings: drawings,
	}
}

func assistantHorizontalLine(anchor dto.AssistantChartDrawingAnchorWriteDto) dto.AssistantChartDrawingWriteDto {
	return dto.AssistantChartDrawingWriteDto{
		Kind: "horizontalLine", Anchors: []dto.AssistantChartDrawingAnchorWriteDto{anchor},
	}
}

func anchorCandle(openTime time.Time) dto.ChartDrawingAnchorCandleDto {
	return dto.ChartDrawingAnchorCandleDto{
		OpenTime: openTime,
		Open:     decimal.RequireFromString("61500"),
		High:     decimal.RequireFromString("62400"),
		Low:      decimal.RequireFromString("61200"),
		Close:    decimal.RequireFromString("61800"),
	}
}

func TestAssistantChartDrawingBatchDomainRefusesWithoutReadingTheMarket(t *testing.T) {
	cases := []struct {
		name        string
		batch       dto.AssistantChartDrawingBatchDto
		refusalText string
	}{
		{name: "an anchor not on the interval's open time",
			batch:       assistantBatch("1h", assistantHorizontalLine(snappedAnchor(assistantAnchorMoment.Add(30*time.Minute), "low"))),
			refusalText: "第 1 個繪圖第 1 個錨點：2026-10-19T08:30:00Z 不是 1h K 線的起始時間"},
		{name: "both a price and a snap",
			batch: assistantBatch("1h", assistantHorizontalLine(dto.AssistantChartDrawingAnchorWriteDto{
				OpenTime: assistantAnchorMoment, SnapTo: "low", Price: decimal.NewNullDecimal(decimal.RequireFromString("1")),
			})),
			refusalText: "價格（price）與吸附（snapTo）要恰好給一種"},
		{name: "neither a price nor a snap",
			batch:       assistantBatch("1h", assistantHorizontalLine(dto.AssistantChartDrawingAnchorWriteDto{OpenTime: assistantAnchorMoment})),
			refusalText: "價格（price）與吸附（snapTo）要恰好給一種"},
		{name: "a two-hour interval",
			batch:       assistantBatch("2h", assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low"))),
			refusalText: "彙總刻度只能是 1m、5m、15m、1h、4h、1d 其中之一"},
		{name: "no interval",
			batch:       assistantBatch("", assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low"))),
			refusalText: "要說出是看哪一種彙總刻度畫的"},
		{name: "an unknown snap",
			batch:       assistantBatch("1h", assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "middle"))),
			refusalText: "吸附只能是 open、high、low、close 其中之一"},
		{name: "a zero price",
			batch:       assistantBatch("1h", assistantHorizontalLine(pricedAnchor(assistantAnchorMoment, "0"))),
			refusalText: "錨點價格必須大於零"},
		{name: "no drawings",
			batch:       assistantBatch("1h"),
			refusalText: "至少要一個繪圖"},
		{name: "the second drawing with the wrong anchor count",
			batch: assistantBatch("1h",
				assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low")),
				dto.AssistantChartDrawingWriteDto{Kind: "trendLine", Anchors: []dto.AssistantChartDrawingAnchorWriteDto{snappedAnchor(assistantAnchorMoment, "low")}}),
			refusalText: "第 2 個繪圖：趨勢線要 2 個錨點，收到 1 個"},
		{name: "an unknown kind",
			batch:       assistantBatch("1h", dto.AssistantChartDrawingWriteDto{Kind: "textNote"}),
			refusalText: "第 1 個繪圖：不認得的繪圖種類"},
		{name: "a style off the palette",
			batch: assistantBatch("1h", dto.AssistantChartDrawingWriteDto{
				Kind: "horizontalLine", Anchors: []dto.AssistantChartDrawingAnchorWriteDto{snappedAnchor(assistantAnchorMoment, "low")},
				Style: dto.ChartDrawingStyleWriteDto{Color: "gold"}}),
			refusalText: "第 1 個繪圖：線色只能是調色盤上的十二色之一"},
		{name: "a contract market spelled wrongly",
			batch: dto.AssistantChartDrawingBatchDto{Place: dto.ChartDrawingBoardPlaceDto{Market: "futures", Symbol: "BTCUSDT"},
				Interval: "1h", Drawings: []dto.AssistantChartDrawingWriteDto{assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low"))}},
			refusalText: "市場只能是現貨（spot）或合約（contract）"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, batchError := domains.NewAssistantChartDrawingBatchDomain(testCase.batch, chartDrawingLimits)

			assert.ErrorIs(t, batchError, domains.ErrChartDrawingValidation)
			assert.ErrorContains(t, batchError, testCase.refusalText)
		})
	}

	batchOfSize := func(size int) dto.AssistantChartDrawingBatchDto {
		drawings := make([]dto.AssistantChartDrawingWriteDto, size)
		for position := range drawings {
			drawings[position] = assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low"))
		}

		return assistantBatch("1h", drawings...)
	}

	t.Run("twenty drawings are one batch", func(t *testing.T) {
		_, batchError := domains.NewAssistantChartDrawingBatchDomain(batchOfSize(20), chartDrawingLimits)

		assert.NoError(t, batchError)
	})

	t.Run("twenty-one drawings are refused", func(t *testing.T) {
		_, batchError := domains.NewAssistantChartDrawingBatchDomain(batchOfSize(21), chartDrawingLimits)

		assert.ErrorIs(t, batchError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, batchError, "一次最多新增 20 個繪圖，收到 21 個")
	})
}

func TestAssistantChartDrawingBatchDomainReadsOneBucketPerAnchorTime(t *testing.T) {
	secondMoment := assistantAnchorMoment.Add(4 * time.Hour)
	batch, batchError := domains.NewAssistantChartDrawingBatchDomain(assistantBatch("4h",
		dto.AssistantChartDrawingWriteDto{Kind: "trendLine", Anchors: []dto.AssistantChartDrawingAnchorWriteDto{
			pricedAnchor(assistantAnchorMoment, "60000"), pricedAnchor(secondMoment, "63000"),
		}},
		assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, "low")),
	), chartDrawingLimits)
	require.NoError(t, batchError)

	assert.Equal(t, []dto.KCandleSeriesQueryDto{
		{Symbol: "BTCUSDT", StartTime: assistantAnchorMoment, EndTime: secondMoment.Add(-time.Nanosecond), Interval: "4h"},
		{Symbol: "BTCUSDT", StartTime: secondMoment, EndTime: secondMoment.Add(4*time.Hour - time.Nanosecond), Interval: "4h"},
	}, batch.AnchorCandleQueries())
	assert.Equal(t, "BTCUSDT", batch.Place().Symbol())
}

func TestAssistantChartDrawingBatchDomainPinsAnchorsToCandles(t *testing.T) {
	t.Run("a snap takes that price of the candle", func(t *testing.T) {
		snapCases := map[string]string{"open": "61500", "high": "62400", "low": "61200", "close": "61800"}
		for snapTo, expectedPrice := range snapCases {
			batch, batchError := domains.NewAssistantChartDrawingBatchDomain(
				assistantBatch("1h", assistantHorizontalLine(snappedAnchor(assistantAnchorMoment, snapTo))), chartDrawingLimits)
			require.NoError(t, batchError)

			drawings, drawingsError := batch.ToDrawings([]dto.ChartDrawingAnchorCandleDto{anchorCandle(assistantAnchorMoment)})

			require.NoError(t, drawingsError)
			assert.Equal(t, expectedPrice, drawings[0].ToDto(1).Anchors[0].Price.String(), snapTo)
		}
	})

	t.Run("a given price is kept and the drawing records the answer it came from", func(t *testing.T) {
		batch, batchError := domains.NewAssistantChartDrawingBatchDomain(
			assistantBatch("1h", assistantHorizontalLine(pricedAnchor(assistantAnchorMoment, "60000"))), chartDrawingLimits)
		require.NoError(t, batchError)

		drawings, drawingsError := batch.ToDrawings([]dto.ChartDrawingAnchorCandleDto{anchorCandle(assistantAnchorMoment)})

		require.NoError(t, drawingsError)
		drawing := drawings[0].ToDto(1)
		assert.Equal(t, "60000", drawing.Anchors[0].Price.String())
		assert.Equal(t, "assistant", drawing.Source)
		assert.Equal(t, uint(12), *drawing.AssistantConversationID)
		assert.Equal(t, uint(34), *drawing.AssistantTurnID)
		assert.Equal(t, "blue", drawing.Color)
		assert.Equal(t, 1, drawing.LineWidth)
	})

	t.Run("a connector batch names no answer", func(t *testing.T) {
		batchDto := assistantBatch("1h", assistantHorizontalLine(pricedAnchor(assistantAnchorMoment, "60000")))
		batchDto.ConversationID = 0
		batchDto.TurnID = 0
		batch, batchError := domains.NewAssistantChartDrawingBatchDomain(batchDto, chartDrawingLimits)
		require.NoError(t, batchError)

		drawings, drawingsError := batch.ToDrawings([]dto.ChartDrawingAnchorCandleDto{anchorCandle(assistantAnchorMoment)})

		require.NoError(t, drawingsError)
		assert.Equal(t, "assistant", drawings[0].ToDto(1).Source)
		assert.Nil(t, drawings[0].ToDto(1).AssistantConversationID)
		assert.Nil(t, drawings[0].ToDto(1).AssistantTurnID)
	})

	t.Run("an anchor without a candle refuses the whole batch", func(t *testing.T) {
		batch, batchError := domains.NewAssistantChartDrawingBatchDomain(assistantBatch("1d",
			assistantHorizontalLine(snappedAnchor(time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), "close"))), chartDrawingLimits)
		require.NoError(t, batchError)

		_, drawingsError := batch.ToDrawings(nil)

		assert.ErrorIs(t, drawingsError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, drawingsError, "第 1 個繪圖第 1 個錨點：2026-10-25T00:00:00Z 沒有 1d K 線")
	})
}
