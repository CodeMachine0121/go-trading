package assistantqueries_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var assistantDrawingHour = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

var assistantDrawingLimits = vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20}

type chartDrawingAssistantQueriesUnderTest struct {
	createQuery                 *assistantqueries.ChartDrawingCreateAssistantQuery
	listQuery                   *assistantqueries.ChartDrawingListAssistantQuery
	removeQuery                 *assistantqueries.ChartDrawingRemoveAssistantQuery
	chartDrawingBoardRepository *mocks.MockIChartDrawingBoardRepository
	kCandleRepository           *mocks.MockIKCandleRepository
}

// newChartDrawingAssistantQueriesUnderTest mocks only storage, so the assistant meets the rules a connector does.
func newChartDrawingAssistantQueriesUnderTest(t *testing.T) chartDrawingAssistantQueriesUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})
	chartDrawingBoardRepository := mocks.NewMockIChartDrawingBoardRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	chartDrawingApplication := application.NewChartDrawingApplication(service.NewChartDrawingService(
		chartDrawingBoardRepository,
		service.NewKCandleService(kCandleRepository, mocks.NewMockIKCandleSummaryDayRepository(controller),
			tradingSymbolRepository, clockProxy, marketCatalog, queryMaxResults),
		service.NewKCandleContractService(mocks.NewMockIKCandleContractRepository(controller),
			mocks.NewMockIKCandleContractSummaryDayRepository(controller), clockProxy, marketCatalog, queryMaxResults),
		assistantDrawingLimits))

	return chartDrawingAssistantQueriesUnderTest{
		createQuery:                 assistantqueries.NewChartDrawingCreateAssistantQuery(chartDrawingApplication, assistantDrawingLimits),
		listQuery:                   assistantqueries.NewChartDrawingListAssistantQuery(chartDrawingApplication),
		removeQuery:                 assistantqueries.NewChartDrawingRemoveAssistantQuery(chartDrawingApplication),
		chartDrawingBoardRepository: chartDrawingBoardRepository,
		kCandleRepository:           kCandleRepository,
	}
}

func assistantBoardDrawing(id uint, source string) dto.ChartDrawingDto {
	return dto.ChartDrawingDto{
		ID: id, Kind: "horizontalLine",
		Anchors:   []dto.ChartDrawingAnchorDto{{OpenTime: assistantDrawingHour, Price: decimal.RequireFromString("100")}},
		Color:     "blue", LineWidth: 1, LineStyle: "solid", Visible: true, Source: source,
	}
}

func (fixture chartDrawingAssistantQueriesUnderTest) boardOfTheViewerIs(revision int64, drawings ...dto.ChartDrawingDto) {
	fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), assistantViewerID, "spot", "BTCUSDT").
		Return(entities.ChartDrawingBoard{
			ID: 3, OwnerID: assistantViewerID, Market: "spot", Symbol: "BTCUSDT",
			Revision: revision, LastDrawingID: uint(len(drawings)), Drawings: drawings,
		}, revision > 0, nil)
}

func (fixture chartDrawingAssistantQueriesUnderTest) savedBoard() *entities.ChartDrawingBoard {
	saved := &entities.ChartDrawingBoard{}
	fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, board entities.ChartDrawingBoard, _ int64) (entities.ChartDrawingBoard, error) {
			*saved = board

			return board, nil
		})

	return saved
}

func TestChartDrawingCreateAssistantQuery(t *testing.T) {
	t.Run("drawings land on the asker's board tied to the answer they came from", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{{
				Symbol: "BTCUSDT", OpenTime: assistantDrawingHour, Open: decimal.RequireFromString("61500"),
				High: decimal.RequireFromString("62400"), Low: decimal.RequireFromString("61200"),
				Close: decimal.RequireFromString("61800"), Volume: decimal.RequireFromString("1"),
			}}, nil)
		fixture.boardOfTheViewerIs(1, assistantBoardDrawing(1, "manual"))
		saved := fixture.savedBoard()

		outcome, runError := fixture.createQuery.Run(t.Context(), assistantOrigin, `{"market":"spot","symbol":"BTCUSDT","interval":"1h",
			"drawings":[{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","snapTo":"low"}]}]}`)

		require.NoError(t, runError)
		assert.JSONEq(t, `{"revision":2,"addedDrawings":[{"id":2,"kind":"horizontalLine",
			"anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"61200"}],"color":"blue","lineWidth":1,"lineStyle":"solid",
			"locked":false,"visible":true,"source":"assistant","assistantConversationId":5,"assistantTurnId":50}]}`, outcome)
		assert.Equal(t, assistantViewerID, saved.OwnerID)
	})

	t.Run("a refusal is a reason the assistant can act on", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		_, runError := fixture.createQuery.Run(t.Context(), assistantOrigin, `{"market":"spot","symbol":"BTCUSDT","interval":"1h",
			"drawings":[{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"1","snapTo":"low"}]}]}`)

		assert.ErrorIs(t, runError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, runError, "價格（price）與吸附（snapTo）要恰好給一種")
	})

	t.Run("arguments that are not JSON are an argument error", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		_, runError := fixture.createQuery.Run(t.Context(), assistantOrigin, `{`)

		assert.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
	})

	t.Run("the description names the batch and board limits", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		assert.Contains(t, fixture.createQuery.Description(), "一次 1 到 20 個")
		assert.Contains(t, fixture.createQuery.Description(), "一份圖最多 200 個")
	})
}

func TestChartDrawingListAssistantQuery(t *testing.T) {
	t.Run("every drawing is listed with who drew it", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)
		fixture.boardOfTheViewerIs(2, assistantBoardDrawing(1, "manual"), assistantBoardDrawing(2, "assistant"))

		outcome, runError := fixture.listQuery.Run(t.Context(), assistantOrigin, `{"market":"spot","symbol":"btcusdt"}`)

		require.NoError(t, runError)
		assert.Contains(t, outcome, `"revision":2`)
		assert.Contains(t, outcome, `"id":1,"kind":"horizontalLine"`)
		assert.Contains(t, outcome, `"source":"manual"`)
		assert.Contains(t, outcome, `"source":"assistant"`)
	})

	t.Run("an unknown market is a reason", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		_, runError := fixture.listQuery.Run(t.Context(), assistantOrigin, `{"market":"futures","symbol":"BTCUSDT"}`)

		assert.ErrorIs(t, runError, domains.ErrChartDrawingValidation)
	})

	t.Run("arguments that are not JSON are an argument error", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		_, runError := fixture.listQuery.Run(t.Context(), assistantOrigin, `[`)

		assert.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
	})
}

func TestChartDrawingRemoveAssistantQuery(t *testing.T) {
	t.Run("only the assistant's drawings go", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)
		fixture.boardOfTheViewerIs(2, assistantBoardDrawing(1, "manual"), assistantBoardDrawing(2, "assistant"))
		saved := fixture.savedBoard()

		outcome, runError := fixture.removeQuery.Run(t.Context(), assistantOrigin, `{"market":"spot","symbol":"BTCUSDT"}`)

		require.NoError(t, runError)
		assert.JSONEq(t, `{"revision":3,"removedCount":1,"removedDrawingIds":[2]}`, outcome)
		require.Len(t, saved.Drawings, 1)
		assert.Equal(t, "manual", saved.Drawings[0].Source)
	})

	t.Run("naming a hand drawing is refused", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)
		fixture.boardOfTheViewerIs(2, assistantBoardDrawing(1, "manual"), assistantBoardDrawing(2, "assistant"))

		_, runError := fixture.removeQuery.Run(t.Context(), assistantOrigin,
			`{"market":"spot","symbol":"BTCUSDT","drawingIds":[1]}`)

		assert.ErrorIs(t, runError, domains.ErrChartDrawingHandDrawn)
	})

	t.Run("arguments that are not JSON are an argument error", func(t *testing.T) {
		fixture := newChartDrawingAssistantQueriesUnderTest(t)

		_, runError := fixture.removeQuery.Run(t.Context(), assistantOrigin, `{`)

		assert.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
	})
}
