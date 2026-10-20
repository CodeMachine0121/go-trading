package domains_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var chartDrawingLimits = vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20}

var chartDrawingMoment = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

func chartDrawingPlace(t *testing.T) domains.ChartDrawingPlaceDomain {
	place, placeError := domains.NewChartDrawingPlaceDomain(
		dto.ChartDrawingBoardPlaceDto{OwnerID: 7, Market: "spot", Symbol: "btcusdt"})
	require.NoError(t, placeError)

	return place
}

func anchorAt(priceText string) dto.ChartDrawingAnchorDto {
	return dto.ChartDrawingAnchorDto{OpenTime: chartDrawingMoment, Price: decimal.RequireFromString(priceText)}
}

func horizontalLineWrite(id uint, priceText string) dto.ChartDrawingWriteDto {
	return dto.ChartDrawingWriteDto{ID: id, Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt(priceText)}}
}

func storedDrawing(id uint, source vo.ChartDrawingSourceVo) dto.ChartDrawingDto {
	conversationID := uint(12)
	turnID := uint(34)
	drawing := dto.ChartDrawingDto{
		ID: id, Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("100")},
		Color: "blue", LineWidth: 1, LineStyle: "solid", Visible: true, Source: string(source),
	}
	if source == vo.ChartDrawingSourceAssistant {
		drawing.AssistantConversationID = &conversationID
		drawing.AssistantTurnID = &turnID
	}

	return drawing
}

func boardOf(t *testing.T, revision int64, drawings ...dto.ChartDrawingDto) domains.ChartDrawingBoardDomain {
	lastDrawingID := uint(0)
	for _, drawing := range drawings {
		lastDrawingID = max(lastDrawingID, drawing.ID)
	}

	return domains.NewChartDrawingBoardDomain(chartDrawingPlace(t), entities.ChartDrawingBoard{
		ID: 3, Revision: revision, LastDrawingID: lastDrawingID, Drawings: drawings,
	}, chartDrawingLimits)
}

func TestChartDrawingPlaceDomain(t *testing.T) {
	t.Run("the symbol is upper-cased and the market kept", func(t *testing.T) {
		place := chartDrawingPlace(t)

		assert.Equal(t, "BTCUSDT", place.Symbol())
		assert.Equal(t, "spot", place.Market())
		assert.False(t, place.IsContract())
	})

	t.Run("a market other than spot or contract is refused", func(t *testing.T) {
		_, placeError := domains.NewChartDrawingPlaceDomain(dto.ChartDrawingBoardPlaceDto{Market: "option", Symbol: "BTCUSDT"})

		assert.ErrorIs(t, placeError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, placeError, "市場只能是現貨（spot）或合約（contract）")
	})

	t.Run("an empty symbol is refused", func(t *testing.T) {
		_, placeError := domains.NewChartDrawingPlaceDomain(dto.ChartDrawingBoardPlaceDto{Market: "contract"})

		assert.ErrorIs(t, placeError, domains.ErrChartDrawingValidation)
	})
}

func TestChartDrawingBoardDomainChecksEveryDrawing(t *testing.T) {
	cases := []struct {
		name        string
		drawing     dto.ChartDrawingWriteDto
		refusalText string
	}{
		{name: "a horizontal line with two anchors",
			drawing:     dto.ChartDrawingWriteDto{Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1"), anchorAt("2")}},
			refusalText: "第 1 個繪圖：水平線要 1 個錨點，收到 2 個"},
		{name: "a rectangle with one anchor",
			drawing:     dto.ChartDrawingWriteDto{Kind: "rectangle", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}},
			refusalText: "矩形要 2 個錨點，收到 1 個"},
		{name: "a zero price",
			drawing:     horizontalLineWrite(0, "0"),
			refusalText: "錨點價格必須大於零"},
		{name: "an anchor without a time",
			drawing:     dto.ChartDrawingWriteDto{Kind: "verticalLine", Anchors: []dto.ChartDrawingAnchorDto{{Price: decimal.RequireFromString("1")}}},
			refusalText: "每個錨點都要有起始時間"},
		{name: "a line width of five",
			drawing:     dto.ChartDrawingWriteDto{Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}, Style: dto.ChartDrawingStyleWriteDto{LineWidth: 5}},
			refusalText: "線寬只能是 1 到 4"},
		{name: "a colour off the palette",
			drawing:     dto.ChartDrawingWriteDto{Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}, Style: dto.ChartDrawingStyleWriteDto{Color: "gold"}},
			refusalText: "線色只能是調色盤上的十二色之一"},
		{name: "an unknown kind",
			drawing:     dto.ChartDrawingWriteDto{Kind: "textNote", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}},
			refusalText: "不認得的繪圖種類「textNote」"},
		{name: "an unknown line style",
			drawing:     dto.ChartDrawingWriteDto{Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}, Style: dto.ChartDrawingStyleWriteDto{LineStyle: "wavy"}},
			refusalText: "線型只能是實線（solid）、虛線（dashed）、點線（dotted）"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name+" refuses the whole board", func(t *testing.T) {
			_, replaceError := boardOf(t, 0).Replace(0, []dto.ChartDrawingWriteDto{testCase.drawing})

			assert.ErrorIs(t, replaceError, domains.ErrChartDrawingValidation)
			assert.ErrorContains(t, replaceError, testCase.refusalText)
		})
	}

	acceptedCases := []struct {
		name    string
		drawing dto.ChartDrawingWriteDto
	}{
		{name: "a trend line with two anchors in blue, width 2, solid", drawing: dto.ChartDrawingWriteDto{
			Kind: "trendLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("60000"), anchorAt("62000")},
			Style: dto.ChartDrawingStyleWriteDto{Color: "blue", LineWidth: 2, LineStyle: "solid"}}},
		{name: "a horizontal line at the smallest positive price", drawing: horizontalLineWrite(0, "0.0001")},
		{name: "a line width of four", drawing: dto.ChartDrawingWriteDto{
			Kind: "horizontalLine", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1")}, Style: dto.ChartDrawingStyleWriteDto{LineWidth: 4}}},
	}

	for _, testCase := range acceptedCases {
		t.Run(testCase.name+" is accepted as given", func(t *testing.T) {
			replacedBoard, replaceError := boardOf(t, 0).Replace(0, []dto.ChartDrawingWriteDto{testCase.drawing})

			require.NoError(t, replaceError)
			savedDrawing := replacedBoard.ToDto().Drawings[0]
			assert.Equal(t, testCase.drawing.Kind, savedDrawing.Kind)
			for position, anchor := range testCase.drawing.Anchors {
				assert.Equal(t, anchor.Price.String(), savedDrawing.Anchors[position].Price.String())
			}
			if testCase.drawing.Style.LineWidth != 0 {
				assert.Equal(t, testCase.drawing.Style.LineWidth, savedDrawing.LineWidth)
			}
		})
	}
}

func TestChartDrawingBoardDomainReplace(t *testing.T) {
	t.Run("the saved board replaces the old one and new drawings are hand drawn with fresh numbers", func(t *testing.T) {
		board := boardOf(t, 3, storedDrawing(1, vo.ChartDrawingSourceManual), storedDrawing(2, vo.ChartDrawingSourceAssistant))

		replacedBoard, replaceError := board.Replace(3, []dto.ChartDrawingWriteDto{
			horizontalLineWrite(1, "100"), horizontalLineWrite(0, "250"),
		})

		require.NoError(t, replaceError)
		boardDto := replacedBoard.ToDto()
		assert.Equal(t, int64(4), boardDto.Revision)
		require.Len(t, boardDto.Drawings, 2)
		assert.Equal(t, uint(1), boardDto.Drawings[0].ID)
		assert.Equal(t, uint(3), boardDto.Drawings[1].ID)
		assert.Equal(t, "manual", boardDto.Drawings[1].Source)
		assert.Nil(t, boardDto.Drawings[1].AssistantTurnID)
	})

	t.Run("an assistant drawing moved by the person stays the assistant's with its answer", func(t *testing.T) {
		board := boardOf(t, 3, storedDrawing(2, vo.ChartDrawingSourceAssistant))

		replacedBoard, replaceError := board.Replace(3, []dto.ChartDrawingWriteDto{horizontalLineWrite(2, "175")})

		require.NoError(t, replaceError)
		movedDrawing := replacedBoard.ToDto().Drawings[0]
		assert.Equal(t, "175", movedDrawing.Anchors[0].Price.String())
		assert.Equal(t, "assistant", movedDrawing.Source)
		assert.Equal(t, uint(12), *movedDrawing.AssistantConversationID)
		assert.Equal(t, uint(34), *movedDrawing.AssistantTurnID)
	})

	t.Run("an unknown number is a new hand drawing with a new number", func(t *testing.T) {
		board := boardOf(t, 1, storedDrawing(4, vo.ChartDrawingSourceAssistant))

		replacedBoard, replaceError := board.Replace(1, []dto.ChartDrawingWriteDto{horizontalLineWrite(99, "100")})

		require.NoError(t, replaceError)
		assert.Equal(t, uint(5), replacedBoard.ToDto().Drawings[0].ID)
		assert.Equal(t, "manual", replacedBoard.ToDto().Drawings[0].Source)
	})

	t.Run("a number listed twice keeps its drawing only once", func(t *testing.T) {
		board := boardOf(t, 1, storedDrawing(4, vo.ChartDrawingSourceAssistant))

		replacedBoard, replaceError := board.Replace(1, []dto.ChartDrawingWriteDto{
			horizontalLineWrite(4, "100"), horizontalLineWrite(4, "200"),
		})

		require.NoError(t, replaceError)
		drawings := replacedBoard.ToDto().Drawings
		assert.Equal(t, []uint{4, 5}, []uint{drawings[0].ID, drawings[1].ID})
		assert.Equal(t, []string{"assistant", "manual"}, []string{drawings[0].Source, drawings[1].Source})
	})

	t.Run("an empty board removes every drawing", func(t *testing.T) {
		board := boardOf(t, 2, storedDrawing(1, vo.ChartDrawingSourceManual), storedDrawing(2, vo.ChartDrawingSourceAssistant))

		replacedBoard, replaceError := board.Replace(2, nil)

		require.NoError(t, replaceError)
		assert.Empty(t, replacedBoard.ToDto().Drawings)
		assert.NotNil(t, replacedBoard.ToDto().Drawings)
		assert.Equal(t, int64(3), replacedBoard.Revision())
	})

	t.Run("a hand drawing anchored tomorrow is saved", func(t *testing.T) {
		tomorrow := dto.ChartDrawingWriteDto{Kind: "verticalLine", Anchors: []dto.ChartDrawingAnchorDto{{
			OpenTime: time.Now().Add(24 * time.Hour), Price: decimal.RequireFromString("1"),
		}}}

		_, replaceError := boardOf(t, 0).Replace(0, []dto.ChartDrawingWriteDto{tomorrow})

		assert.NoError(t, replaceError)
	})

	t.Run("a board changed since the given revision is refused with the board as it is", func(t *testing.T) {
		board := boardOf(t, 4, storedDrawing(1, vo.ChartDrawingSourceAssistant))

		_, replaceError := board.Replace(3, nil)

		assert.ErrorIs(t, replaceError, domains.ErrChartDrawingBoardRevisionConflict)
		revisionConflictError, isRevisionConflict := errors.AsType[domains.ChartDrawingBoardRevisionConflictError](replaceError)
		require.True(t, isRevisionConflict)
		assert.Equal(t, int64(4), revisionConflictError.CurrentBoard.Revision)
		assert.Len(t, revisionConflictError.CurrentBoard.Drawings, 1)
		assert.Contains(t, replaceError.Error(), "繪圖板在你讀取之後已經變了（現在是第 4 版）")
	})

	t.Run("exactly the capacity is saved and one more is refused", func(t *testing.T) {
		atCapacity := make([]dto.ChartDrawingWriteDto, 200)
		for position := range atCapacity {
			atCapacity[position] = horizontalLineWrite(0, "1")
		}

		_, fullError := boardOf(t, 0).Replace(0, atCapacity)
		_, overError := boardOf(t, 0).Replace(0, append(atCapacity, horizontalLineWrite(0, "1")))

		assert.NoError(t, fullError)
		assert.ErrorIs(t, overError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, overError, "一份繪圖板最多 200 個繪圖，收到 201 個")
	})

	t.Run("a style not given takes the kind's default", func(t *testing.T) {
		locked := true
		replacedBoard, replaceError := boardOf(t, 0).Replace(0, []dto.ChartDrawingWriteDto{
			{Kind: "rectangle", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1"), anchorAt("2")}},
			{Kind: "fibonacciRetracement", Anchors: []dto.ChartDrawingAnchorDto{anchorAt("1"), anchorAt("2")},
				Style: dto.ChartDrawingStyleWriteDto{Locked: &locked}},
		})

		require.NoError(t, replaceError)
		drawings := replacedBoard.ToDto().Drawings
		assert.Equal(t, []string{"purple", "gray"}, []string{drawings[0].Color, drawings[1].Color})
		assert.Equal(t, "solid", drawings[0].LineStyle)
		assert.False(t, drawings[0].Locked)
		assert.True(t, drawings[0].Visible)
		assert.True(t, drawings[1].Locked)
	})
}

func TestChartDrawingBoardDomainAppendAssistantDrawings(t *testing.T) {
	assistantDrawings := func(t *testing.T, count int) []domains.ChartDrawingDomain {
		drawings := make([]domains.ChartDrawingDomain, 0, count)
		for range count {
			drawing, drawingError := domains.NewChartDrawingDomain("horizontalLine",
				[]dto.ChartDrawingAnchorDto{anchorAt("1")}, dto.ChartDrawingStyleWriteDto{},
				vo.ChartDrawingProvenanceVo{Source: vo.ChartDrawingSourceAssistant})
			require.NoError(t, drawingError)
			drawings = append(drawings, drawing)
		}

		return drawings
	}
	boardWith := func(t *testing.T, count int) domains.ChartDrawingBoardDomain {
		drawings := make([]dto.ChartDrawingDto, 0, count)
		for position := range count {
			drawings = append(drawings, storedDrawing(uint(position+1), vo.ChartDrawingSourceManual))
		}

		return boardOf(t, 2, drawings...)
	}

	t.Run("the batch goes on top and the revision moves on", func(t *testing.T) {
		appendedBoard, addedDrawings, appendError := boardWith(t, 5).AppendAssistantDrawings(assistantDrawings(t, 3))

		require.NoError(t, appendError)
		assert.Len(t, appendedBoard.ToDto().Drawings, 8)
		assert.Equal(t, int64(3), appendedBoard.Revision())
		assert.Equal(t, []uint{6, 7, 8}, []uint{addedDrawings[0].ID, addedDrawings[1].ID, addedDrawings[2].ID})
		assert.Equal(t, addedDrawings, appendedBoard.ToDto().Drawings[5:])
		assert.Equal(t, "assistant", addedDrawings[0].Source)
	})

	t.Run("filling the board exactly is accepted", func(t *testing.T) {
		appendedBoard, _, appendError := boardWith(t, 190).AppendAssistantDrawings(assistantDrawings(t, 10))

		require.NoError(t, appendError)
		assert.Len(t, appendedBoard.ToDto().Drawings, 200)
	})

	t.Run("one more than the free places refuses the whole batch", func(t *testing.T) {
		_, _, appendError := boardWith(t, 190).AppendAssistantDrawings(assistantDrawings(t, 11))

		assert.ErrorIs(t, appendError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, appendError, "這份只剩 10 個空位")
	})
}

func TestChartDrawingBoardDomainRemoveAssistantDrawings(t *testing.T) {
	boardWithBoth := func(t *testing.T) domains.ChartDrawingBoardDomain {
		return boardOf(t, 5, storedDrawing(1, vo.ChartDrawingSourceManual),
			storedDrawing(2, vo.ChartDrawingSourceAssistant), storedDrawing(3, vo.ChartDrawingSourceAssistant))
	}
	remainingIDs := func(board domains.ChartDrawingBoardDomain) []uint {
		ids := make([]uint, 0)
		for _, drawing := range board.ToDto().Drawings {
			ids = append(ids, drawing.ID)
		}

		return ids
	}

	t.Run("naming none removes every assistant drawing", func(t *testing.T) {
		remainingBoard, removedDrawingIDs, removeError := boardWithBoth(t).RemoveAssistantDrawings(nil)

		require.NoError(t, removeError)
		assert.Equal(t, []uint{2, 3}, removedDrawingIDs)
		assert.Equal(t, []uint{1}, remainingIDs(remainingBoard))
		assert.Equal(t, int64(6), remainingBoard.Revision())
	})

	t.Run("naming one removes only that one", func(t *testing.T) {
		remainingBoard, removedDrawingIDs, removeError := boardWithBoth(t).RemoveAssistantDrawings([]uint{2})

		require.NoError(t, removeError)
		assert.Equal(t, []uint{2}, removedDrawingIDs)
		assert.Equal(t, []uint{1, 3}, remainingIDs(remainingBoard))
	})

	t.Run("naming a hand drawing removes nothing", func(t *testing.T) {
		_, _, removeError := boardWithBoth(t).RemoveAssistantDrawings([]uint{1, 2})

		assert.ErrorIs(t, removeError, domains.ErrChartDrawingHandDrawn)
		assert.ErrorContains(t, removeError, "編號 1 的繪圖是使用者手畫的，助手不能移除")
	})

	t.Run("naming an unknown drawing removes nothing", func(t *testing.T) {
		_, _, removeError := boardWithBoth(t).RemoveAssistantDrawings([]uint{9})

		assert.ErrorIs(t, removeError, domains.ErrChartDrawingNotFound)
		assert.ErrorContains(t, removeError, "這份繪圖板上沒有編號 9 的繪圖")
	})

	t.Run("nothing of the assistant's leaves the board and its revision alone", func(t *testing.T) {
		board := boardOf(t, 5, storedDrawing(1, vo.ChartDrawingSourceManual))

		remainingBoard, removedDrawingIDs, removeError := board.RemoveAssistantDrawings(nil)

		require.NoError(t, removeError)
		assert.Empty(t, removedDrawingIDs)
		assert.Equal(t, []uint{1}, remainingIDs(remainingBoard))
		assert.Equal(t, int64(5), remainingBoard.Revision())
	})
}

func TestChartDrawingBoardDomainToEntity(t *testing.T) {
	t.Run("the row carries the place, the next revision and the numbering", func(t *testing.T) {
		replacedBoard, replaceError := boardOf(t, 1, storedDrawing(4, vo.ChartDrawingSourceManual)).
			Replace(1, []dto.ChartDrawingWriteDto{horizontalLineWrite(0, "1")})
		require.NoError(t, replaceError)

		boardEntity := replacedBoard.ToEntity()

		assert.Equal(t, uint(3), boardEntity.ID)
		assert.Equal(t, uint(7), boardEntity.OwnerID)
		assert.Equal(t, "spot", boardEntity.Market)
		assert.Equal(t, "BTCUSDT", boardEntity.Symbol)
		assert.Equal(t, int64(2), boardEntity.Revision)
		assert.Equal(t, uint(5), boardEntity.LastDrawingID)
		assert.Len(t, boardEntity.Drawings, 1)
	})
}
