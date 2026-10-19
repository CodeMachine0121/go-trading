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

type screeningRouterUnderTest struct {
	engine             *gin.Engine
	profileRepository  *mocks.MockISymbolScreeningProfileRepository
	runRepository      *mocks.MockISymbolScreeningRunRepository
	strategyScriptRepo *mocks.MockIStrategyScriptRepository
}

func newScreeningRouterUnderTest(t *testing.T) screeningRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(mockController)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()
	underTest := screeningRouterUnderTest{
		profileRepository:  mocks.NewMockISymbolScreeningProfileRepository(mockController),
		runRepository:      mocks.NewMockISymbolScreeningRunRepository(mockController),
		strategyScriptRepo: mocks.NewMockIStrategyScriptRepository(mockController),
	}
	candidateRepository := mocks.NewMockISymbolScreeningCandidateRepository(mockController)
	candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	marketDataService := service.NewSymbolScreeningMarketDataService(
		mocks.NewMockISymbolScreeningKCandleRepository(mockController),
		mocks.NewMockISymbolScreeningFundingRateSettlementRepository(mockController),
		mocks.NewMockIContractMarketDataProxy(mockController), mocks.NewMockIContractSymbolLookupProxy(mockController),
		mocks.NewMockIContractFundingRateProxy(mockController), mocks.NewMockIContractMaintenanceMarginTierProxy(mockController),
		clockProxy)
	screeningController := controller.NewSymbolScreeningController(application.NewSymbolScreeningApplication(
		service.NewSymbolScreeningProfileService(underTest.profileRepository, underTest.runRepository,
			service.NewStrategyScriptService(underTest.strategyScriptRepo,
				mocks.NewMockIPublishedStrategyScriptRepository(mockController))),
		service.NewSymbolScreeningRunService(underTest.runRepository, candidateRepository, transactionRepository,
			clockProxy, marketDataService, nil, "replica-a", 20, 2*time.Hour, 0),
		marketDataService, 90*24*time.Hour, 30*24*time.Hour))

	door := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.POST("/symbol-screening-profiles", door, screeningController.CreateProfile)
	engine.DELETE("/symbol-screening-profiles/:id", door, screeningController.DeleteProfile)
	engine.POST("/symbol-screening-runs", door, screeningController.SubmitRun)
	engine.GET("/symbol-screening-runs/:id", door, screeningController.GetRun)
	engine.POST("/symbol-screening-runs/:id/cancel", door, screeningController.CancelRun)
	underTest.engine = engine

	return underTest
}

func (underTest screeningRouterUnderTest) send(method string, target string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", signedInProof)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	underTest.engine.ServeHTTP(recorder, request)

	return recorder
}

func (underTest screeningRouterUnderTest) theProfileAndScriptAreMine() {
	underTest.strategyScriptRepo.EXPECT().FindOne(gomock.Any(), uint(108)).Return(entities.StrategyScript{
		ID: 108, OwnerID: signedInViewerID, Script: "the script", ResultType: "signal",
		MarketDataKind: string(vo.MarketDataKindContractKCandle),
	}, nil).AnyTimes()
	underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(4)).
		Return(entities.SymbolScreeningProfile{
			ID: 4, OwnerID: signedInViewerID, Name: "趨勢 V3", StrategyScriptID: 108, AggregationInterval: "4h",
			ParameterValues: []dto.StrategyScriptParameterValueDto{}, TradingMode: "longOnly",
			Leverage: decimal.NewFromInt(1), HistoryDays: 730, SegmentCount: 3, MinimumTradeCount: 6,
		}, true, nil).AnyTimes()
	underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil).AnyTimes()
}

func TestSubmitSymbolScreeningRunEndpoint(t *testing.T) {
	t.Run("an accepted run answers 202 while it waits in the queue", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.theProfileAndScriptAreMine()
		underTest.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), signedInViewerID).Return(0, nil)
		underTest.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, run entities.SymbolScreeningRun) (entities.SymbolScreeningRun, error) {
				run.ID = 9
				return run, nil
			})

		recorder := underTest.send(http.MethodPost, "/symbol-screening-runs", `{"profileId":4,"symbols":"xrpusdt,linkusdt"}`)

		require.Equal(t, http.StatusAccepted, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"status":"queued"`)
		assert.Contains(t, recorder.Body.String(), `"candidateCount":2`)
	})

	t.Run("a run of mine already waiting answers 429", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.theProfileAndScriptAreMine()
		underTest.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), signedInViewerID).Return(1, nil)

		recorder := underTest.send(http.MethodPost, "/symbol-screening-runs", `{"profileId":4,"symbols":"XRPUSDT"}`)

		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
		assert.JSONEq(t, `{"message":"你已經有一筆篩選在排隊或進行中，等它結束再送"}`, recorder.Body.String())
	})

	t.Run("an unreadable list answers 400 naming the symbol", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.theProfileAndScriptAreMine()

		recorder := underTest.send(http.MethodPost, "/symbol-screening-runs", `{"profileId":4,"symbols":"BTC-USDT"}`)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "BTC-USDT 只能有英文字母與數字")
	})
}

func TestSymbolScreeningRunEndpoints(t *testing.T) {
	t.Run("an identifier that is not a positive whole number answers 400", func(t *testing.T) {
		recorder := newScreeningRouterUnderTest(t).send(http.MethodGet, "/symbol-screening-runs/abc", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("someone else's run answers 404", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(70)).
			Return(entities.SymbolScreeningRun{}, false, nil)

		recorder := underTest.send(http.MethodGet, "/symbol-screening-runs/70", "")

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("cancelling an ended run answers 409", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(9)).
			Return(entities.SymbolScreeningRun{ID: 9, OwnerID: signedInViewerID, Status: "succeeded"}, true, nil)

		recorder := underTest.send(http.MethodPost, "/symbol-screening-runs/9/cancel", "")

		require.Equal(t, http.StatusConflict, recorder.Code)
		assert.JSONEq(t, `{"message":"這筆篩選已經結束，不能取消"}`, recorder.Body.String())
	})
}

func TestSymbolScreeningProfileEndpoints(t *testing.T) {
	t.Run("a new profile answers 201", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.theProfileAndScriptAreMine()
		underTest.profileRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, profile entities.SymbolScreeningProfile) (entities.SymbolScreeningProfile, error) {
				profile.ID = 5
				return profile, nil
			})

		recorder := underTest.send(http.MethodPost, "/symbol-screening-profiles",
			`{"name":"趨勢 V3","strategyScriptId":108,"aggregationInterval":"4h"}`)

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"historyDays":730`)
	})

	t.Run("a fifteen-minute interval answers 400 with why", func(t *testing.T) {
		recorder := newScreeningRouterUnderTest(t).send(http.MethodPost, "/symbol-screening-profiles",
			`{"name":"太細","strategyScriptId":108,"aggregationInterval":"15m"}`)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "看盤間隔只能是每 4 小時或每天")
	})

	t.Run("a profile a waiting run uses answers 409 on delete", func(t *testing.T) {
		underTest := newScreeningRouterUnderTest(t)
		underTest.theProfileAndScriptAreMine()
		underTest.runRepository.EXPECT().CountActiveByProfile(gomock.Any(), uint(4)).Return(1, nil)

		recorder := underTest.send(http.MethodDelete, "/symbol-screening-profiles/4", "")

		require.Equal(t, http.StatusConflict, recorder.Code)
		assert.JSONEq(t, `{"message":"有篩選正在使用這份設定，等它結束再刪除"}`, recorder.Body.String())
	})

	t.Run("an unreadable body answers 400", func(t *testing.T) {
		recorder := newScreeningRouterUnderTest(t).send(http.MethodPost, "/symbol-screening-profiles", `{not json`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}
