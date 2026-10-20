package controller_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var chartDrawingRouteHour = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

type chartDrawingRouterUnderTest struct {
	engine                      *gin.Engine
	chartDrawingBoardRepository *mocks.MockIChartDrawingBoardRepository
	kCandleRepository           *mocks.MockIKCandleRepository
}

func newChartDrawingRouterUnderTest(t *testing.T) chartDrawingRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})
	fixture := chartDrawingRouterUnderTest{
		chartDrawingBoardRepository: mocks.NewMockIChartDrawingBoardRepository(mockController),
		kCandleRepository:           mocks.NewMockIKCandleRepository(mockController),
	}
	chartDrawingController := controller.NewChartDrawingController(application.NewChartDrawingApplication(
		service.NewChartDrawingService(
			fixture.chartDrawingBoardRepository,
			service.NewKCandleService(fixture.kCandleRepository, mocks.NewMockIKCandleSummaryDayRepository(mockController),
				tradingSymbolRepository, clockProxy, marketCatalog, 1000),
			service.NewKCandleContractService(mocks.NewMockIKCandleContractRepository(mockController),
				mocks.NewMockIKCandleContractSummaryDayRepository(mockController), clockProxy, marketCatalog, 1000),
			vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20})))

	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.GET("/users/me/chart-drawings/:market/:symbol", requiresSignIn, chartDrawingController.GetBoard)
	engine.PUT("/users/me/chart-drawings/:market/:symbol", requiresSignIn, chartDrawingController.ReplaceBoard)
	engine.POST("/users/me/chart-drawings/:market/:symbol/assistant-drawings", requiresSignIn,
		chartDrawingController.AddAssistantDrawings)
	engine.DELETE("/users/me/chart-drawings/:market/:symbol/assistant-drawings", requiresSignIn,
		chartDrawingController.RemoveAssistantDrawings)
	fixture.engine = engine

	return fixture
}

func (fixture chartDrawingRouterUnderTest) send(method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func (fixture chartDrawingRouterUnderTest) boardIs(board entities.ChartDrawingBoard) {
	fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), signedInViewerID, gomock.Any(), "BTCUSDT").
		Return(board, board.ID != 0, nil).AnyTimes()
}

func (fixture chartDrawingRouterUnderTest) savesEcho() {
	fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, board entities.ChartDrawingBoard, _ int64) (entities.ChartDrawingBoard, error) {
			return board, nil
		}).AnyTimes()
}

func routedDrawing(id uint, source string) dto.ChartDrawingDto {
	conversationID := uint(12)
	turnID := uint(34)
	drawing := dto.ChartDrawingDto{
		ID: id, Kind: "horizontalLine",
		Anchors:   []dto.ChartDrawingAnchorDto{{OpenTime: chartDrawingRouteHour, Price: decimal.RequireFromString("61200")}},
		Color:     "blue", LineWidth: 1, LineStyle: "solid", Visible: true, Source: source,
	}
	if source == "assistant" {
		drawing.AssistantConversationID = &conversationID
		drawing.AssistantTurnID = &turnID
	}

	return drawing
}

func routedBoard(revision int64, drawings ...dto.ChartDrawingDto) entities.ChartDrawingBoard {
	return entities.ChartDrawingBoard{
		ID: 3, OwnerID: signedInViewerID, Market: "spot", Symbol: "BTCUSDT",
		Revision: revision, LastDrawingID: uint(len(drawings)), Drawings: drawings,
	}
}

func TestChartDrawingRouterReadsABoard(t *testing.T) {
	t.Run("the board answers in the agreed shape", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(3, routedDrawing(1, "assistant")))

		response := fixture.send(http.MethodGet, "/users/me/chart-drawings/spot/btcusdt", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"market":"spot","symbol":"BTCUSDT","revision":3,"drawings":[
			{"id":1,"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"61200"}],
			 "color":"blue","lineWidth":1,"lineStyle":"solid","locked":false,"visible":true,
			 "source":"assistant","assistantConversationId":12,"assistantTurnId":34}]}`, response.Body.String())
	})

	t.Run("a board never drawn on lists no drawings", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(entities.ChartDrawingBoard{})

		response := fixture.send(http.MethodGet, "/users/me/chart-drawings/contract/BTCUSDT", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"market":"contract","symbol":"BTCUSDT","revision":0,"drawings":[]}`, response.Body.String())
	})

	t.Run("an unknown market is a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		response := fixture.send(http.MethodGet, "/users/me/chart-drawings/option/BTCUSDT", "")

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "市場只能是現貨（spot）或合約（contract）")
	})

	t.Run("a failed read is a bad gateway", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ChartDrawingBoard{}, false, context.DeadlineExceeded)

		assert.Equal(t, http.StatusBadGateway, fixture.send(http.MethodGet, "/users/me/chart-drawings/spot/BTCUSDT", "").Code)
	})
}

func TestChartDrawingRouterSavesABoardWhole(t *testing.T) {
	t.Run("the saved board comes back with the system's sources and numbers", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(3, routedDrawing(1, "assistant")))
		fixture.savesEcho()

		response := fixture.send(http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT", `{"revision":3,"drawings":[
			{"id":1,"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"61000"}],"source":"manual"},
			{"kind":"trendLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":60000},{"openTime":"2026-10-21T08:00:00Z","price":"62000"}],
			 "color":"red","lineWidth":3,"lineStyle":"dashed","locked":true,"visible":false}]}`)

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"market":"spot","symbol":"BTCUSDT","revision":4,"drawings":[
			{"id":1,"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"61000"}],
			 "color":"blue","lineWidth":1,"lineStyle":"solid","locked":false,"visible":true,
			 "source":"assistant","assistantConversationId":12,"assistantTurnId":34},
			{"id":2,"kind":"trendLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"60000"},{"openTime":"2026-10-21T08:00:00Z","price":"62000"}],
			 "color":"red","lineWidth":3,"lineStyle":"dashed","locked":true,"visible":false,
			 "source":"manual","assistantConversationId":null,"assistantTurnId":null}]}`, response.Body.String())
	})

	t.Run("a board changed since is a conflict carrying the board as it is", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(4, routedDrawing(1, "assistant")))

		response := fixture.send(http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT", `{"revision":3,"drawings":[]}`)

		require.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "繪圖板在你讀取之後已經變了（現在是第 4 版）")
		assert.Contains(t, response.Body.String(), `"board":{"market":"spot","symbol":"BTCUSDT","revision":4`)
	})

	t.Run("a missing revision is a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT", `{"drawings":[]}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "revision")
	})

	t.Run("a drawing breaking the rules is a bad request naming it", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(entities.ChartDrawingBoard{})

		response := fixture.send(http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT", `{"revision":0,"drawings":[
			{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"1"},{"openTime":"2026-10-19T08:00:00Z","price":"2"}]}]}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "第 1 個繪圖：水平線要 1 個錨點，收到 2 個")
	})
}

func TestChartDrawingRouterAssistantDrawings(t *testing.T) {
	t.Run("a batch is drawn as the assistant's and answered as created", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{{
				Symbol: "BTCUSDT", OpenTime: chartDrawingRouteHour, Open: decimal.RequireFromString("61500"),
				High: decimal.RequireFromString("62400"), Low: decimal.RequireFromString("61200"),
				Close: decimal.RequireFromString("61800"), Volume: decimal.RequireFromString("1"),
			}}, nil)
		fixture.boardIs(routedBoard(2, routedDrawing(1, "manual")))
		fixture.savesEcho()

		response := fixture.send(http.MethodPost, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings", `{"interval":"1h","drawings":[
			{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","snapTo":"high"}],"color":"red","lineWidth":2,"lineStyle":"dashed"}]}`)

		require.Equal(t, http.StatusCreated, response.Code)
		assert.JSONEq(t, `{"revision":3,"addedDrawings":[
			{"id":2,"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"62400"}],
			 "color":"red","lineWidth":2,"lineStyle":"dashed","locked":false,"visible":true,
			 "source":"assistant","assistantConversationId":null,"assistantTurnId":null}]}`, response.Body.String())
	})

	t.Run("an anchor off the interval is a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings", `{"interval":"1h","drawings":[
			{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:30:00Z","price":"1"}]}]}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "不是 1h K 線的起始時間")
	})

	t.Run("a symbol carrying a NUL character is a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/users/me/chart-drawings/spot/BTC%00USDT/assistant-drawings", `{"interval":"1h","drawings":[
			{"kind":"horizontalLine","anchors":[{"openTime":"2026-10-19T08:00:00Z","price":"1"}]}]}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("a body that is not JSON is a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		assert.Equal(t, http.StatusBadRequest,
			fixture.send(http.MethodPost, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings", `{`).Code)
	})

	t.Run("removing names what went", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(5, routedDrawing(1, "manual"), routedDrawing(2, "assistant"), routedDrawing(3, "assistant")))
		fixture.savesEcho()

		response := fixture.send(http.MethodDelete, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings?ids=3", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"revision":6,"removedCount":1,"removedDrawingIds":[3]}`, response.Body.String())
	})

	t.Run("removing a hand drawing is a conflict", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(5, routedDrawing(1, "manual"), routedDrawing(2, "assistant")))

		response := fixture.send(http.MethodDelete, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings?ids=1,2", "")

		assert.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "編號 1 的繪圖是使用者手畫的，助手不能移除")
	})

	t.Run("removing an unknown drawing is not found", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)
		fixture.boardIs(routedBoard(5, routedDrawing(1, "manual")))

		response := fixture.send(http.MethodDelete, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings?ids=9", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("identifiers that are not positive whole numbers are a bad request", func(t *testing.T) {
		fixture := newChartDrawingRouterUnderTest(t)

		for _, ids := range []string{"0", "abc", "-1", "1,x"} {
			response := fixture.send(http.MethodDelete, "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings?ids="+ids, "")

			assert.Equal(t, http.StatusBadRequest, response.Code, ids)
		}
	})
}
