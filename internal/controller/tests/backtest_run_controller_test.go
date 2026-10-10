package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type backtestRunRouterUnderTest struct {
	engine     *gin.Engine
	runs       routerBacktestRuns
	clockProxy *mocks.MockIClockProxy
}

func newBacktestRunRouterUnderTest(t *testing.T) backtestRunRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	runs := newRouterBacktestRuns(t, mockController, clockProxy, nil, nil)
	backtestRunController := controller.NewBacktestRunController(
		application.NewBacktestRunApplication(runs.backtestRunService, 2, 30*24*time.Hour))

	engine := gin.New()
	engine.GET("/backtest-runs", doorOpenFor(t, signedInViewerID), backtestRunController.ListRuns)
	engine.GET("/backtest-runs/:id", doorOpenFor(t, signedInViewerID), backtestRunController.GetRun)
	engine.POST("/backtest-runs/:id/cancel", doorOpenFor(t, signedInViewerID), backtestRunController.CancelRun)

	return backtestRunRouterUnderTest{engine: engine, runs: runs, clockProxy: clockProxy}
}

func (fixture backtestRunRouterUnderTest) send(method string, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func aStoredBacktestRun(id uint, status vo.BacktestRunStatusVo) entities.BacktestRun {
	return entities.BacktestRun{
		ID: id, OwnerID: signedInViewerID, Kind: string(vo.BacktestRunSpotScript), Symbol: "BTCUSDT", Interval: "1h",
		StartTime: backtestRouterStart, EndTime: backtestRouterStart.Add(4 * time.Hour),
		Status: string(status), SubmittedAt: backtestRouterNow,
	}
}

func aSucceededBacktestRun(id uint, kind vo.BacktestRunKindVo) entities.BacktestRun {
	backtestRun := aStoredBacktestRun(id, vo.BacktestRunSucceeded)
	backtestRun.Kind = string(kind)
	totalReturnRate, maximumDrawdown, tradeCount := 0.255, 0.101, 4
	finishedAt := backtestRouterNow.Add(time.Hour)
	backtestRun.TotalReturnRate, backtestRun.MaximumDrawdown, backtestRun.TradeCount =
		&totalReturnRate, &maximumDrawdown, &tradeCount
	backtestRun.FinishedAt = &finishedAt
	if kind == vo.BacktestRunContractScript {
		backtestRun.Leverage = decimal.NewNullDecimal(decimal.RequireFromString("5"))
		backtestRun.TradingMode = "longShort"
	}

	return backtestRun
}

func TestGetBacktestRunEndpointAnswersTheReportExactlyAsTheReplayWrites(t *testing.T) {
	winRate := 0.5
	spotReport := dto.BacktestResultDto{
		Symbol: "BTCUSDT", Interval: "1h", UsedCandleCount: 4,
		Summary:      dto.BacktestSummaryDto{FinalEquity: decimal.RequireFromString("12550"), TotalReturnRate: 0.255, WinRate: &winRate},
		ClosedTrades: []dto.ClosedTradeDto{},
		EquityCurve:  []dto.EquityPointDto{{OpenTime: backtestRouterStart, Equity: decimal.RequireFromString("10000")}},
		FillTiming:   "close",
	}
	contractReport := dto.ContractBacktestResultDto{
		Symbol: "BTCUSDT", Interval: "1h", TradingMode: "longShort", Leverage: decimal.NewFromInt(5),
		ClosedTrades: []dto.ContractClosedTradeDto{}, EquityCurve: []dto.EquityPointDto{}, FillTiming: "close",
	}
	testCases := []struct {
		name         string
		backtestRun  entities.BacktestRun
		result       entities.BacktestRunResult
		expectedJSON any
		leverage     any
		tradingMode  any
	}{
		{
			name:         "a spot run",
			backtestRun:  aSucceededBacktestRun(12, vo.BacktestRunSpotScript),
			result:       entities.BacktestRunResult{BacktestRunID: 12, SpotResult: &spotReport},
			expectedJSON: spotReport, leverage: nil, tradingMode: nil,
		},
		{
			name:         "a contract run",
			backtestRun:  aSucceededBacktestRun(12, vo.BacktestRunContractScript),
			result:       entities.BacktestRunResult{BacktestRunID: 12, ContractResult: &contractReport},
			expectedJSON: contractReport, leverage: "5", tradingMode: "longShort",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunRouterUnderTest(t)
			fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
			fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(12)).
				Return(testCase.backtestRun, true, nil)
			fixture.runs.resultRepository.EXPECT().FindOne(gomock.Any(), uint(12)).Return(testCase.result, true, nil)

			response := fixture.send(http.MethodGet, "/backtest-runs/12")

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			body := map[string]json.RawMessage{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			expectedResult, marshalError := json.Marshal(testCase.expectedJSON)
			require.NoError(t, marshalError)
			assert.JSONEq(t, string(expectedResult), string(body["result"]))
			fields := map[string]any{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &fields))
			assert.Equal(t, "succeeded", fields["status"])
			assert.Equal(t, float64(100), fields["progressPercent"])
			assert.Equal(t, 0.255, fields["totalReturnRate"])
			assert.Equal(t, 0.101, fields["maximumDrawdown"])
			assert.Equal(t, float64(4), fields["tradeCount"])
			assert.Equal(t, testCase.leverage, fields["leverage"])
			assert.Equal(t, testCase.tradingMode, fields["tradingMode"])
		})
	}
}

func TestGetBacktestRunEndpointForARunNotYetDone(t *testing.T) {
	testCases := []struct {
		name            string
		backtestRun     func() entities.BacktestRun
		expectedStatus  string
		expectedPercent float64
		expectedReason  string
	}{
		{
			name:           "a queued run has no report",
			backtestRun:    func() entities.BacktestRun { return aStoredBacktestRun(12, vo.BacktestRunQueued) },
			expectedStatus: "queued",
		},
		{
			name: "a running run says how far it has got",
			backtestRun: func() entities.BacktestRun {
				backtestRun := aStoredBacktestRun(13, vo.BacktestRunRunning)
				backtestRun.CompletedBarCount, backtestRun.TotalBarCount = 120, 480
				return backtestRun
			},
			expectedStatus: "running", expectedPercent: 25,
		},
		{
			name: "a failed run keeps why",
			backtestRun: func() entities.BacktestRun {
				backtestRun := aStoredBacktestRun(14, vo.BacktestRunFailed)
				backtestRun.FailureReason = "超過時間上限"
				return backtestRun
			},
			expectedStatus: "failed", expectedReason: "超過時間上限",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunRouterUnderTest(t)
			fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
			backtestRun := testCase.backtestRun()
			fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, backtestRun.ID).
				Return(backtestRun, true, nil)

			response := fixture.send(http.MethodGet, "/backtest-runs/"+jsonNumber(backtestRun.ID))

			require.Equal(t, http.StatusOK, response.Code)
			body := map[string]any{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			result, hasResult := body["result"]
			assert.True(t, hasResult)
			assert.Nil(t, result)
			assert.Equal(t, testCase.expectedStatus, body["status"])
			assert.Equal(t, testCase.expectedPercent, body["progressPercent"])
			assert.Equal(t, testCase.expectedReason, body["failureReason"])
			assert.Nil(t, body["totalReturnRate"])
		})
	}
}

func TestGetBacktestRunEndpointWaitsUntilTheRunEnds(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	clock := backtestRouterNow
	fixture.clockProxy.EXPECT().Now().DoAndReturn(func() time.Time { return clock }).AnyTimes()
	fixture.clockProxy.EXPECT().Sleep(time.Second).DoAndReturn(func(duration time.Duration) { clock = clock.Add(duration) }).Times(2)
	running := aStoredBacktestRun(12, vo.BacktestRunRunning)
	failed := aStoredBacktestRun(12, vo.BacktestRunFailed)
	gomock.InOrder(
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(12)).Return(running, true, nil).Times(2),
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(12)).Return(failed, true, nil),
	)

	response := fixture.send(http.MethodGet, "/backtest-runs/12?waitSeconds=30")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"failed"`)
}

func TestGetBacktestRunEndpointGivesUpWaitingAfterAMinuteAtMost(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	clock := backtestRouterNow
	fixture.clockProxy.EXPECT().Now().DoAndReturn(func() time.Time { return clock }).AnyTimes()
	sleeps := 0
	fixture.clockProxy.EXPECT().Sleep(time.Second).DoAndReturn(func(duration time.Duration) {
		sleeps++
		clock = clock.Add(duration)
	}).AnyTimes()
	fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(12)).
		Return(aStoredBacktestRun(12, vo.BacktestRunRunning), true, nil).AnyTimes()

	response := fixture.send(http.MethodGet, "/backtest-runs/12?waitSeconds=600")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"running"`)
	assert.Equal(t, 60, sleeps)
}

func TestGetBacktestRunEndpointForARunThatIsNotTheViewers(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
	fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(16)).
		Return(entities.BacktestRun{}, false, nil)

	response := fixture.send(http.MethodGet, "/backtest-runs/16")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestListBacktestRunsEndpoint(t *testing.T) {
	testCases := []struct {
		name           string
		target         string
		expectedFilter dto.BacktestRunListQueryDto
	}{
		{name: "no limit is twenty", target: "/backtest-runs", expectedFilter: dto.BacktestRunListQueryDto{Limit: 20}},
		{name: "five hundred is two hundred", target: "/backtest-runs?limit=500",
			expectedFilter: dto.BacktestRunListQueryDto{Limit: 200}},
		{name: "an unreadable limit is twenty", target: "/backtest-runs?limit=many",
			expectedFilter: dto.BacktestRunListQueryDto{Limit: 20}},
		{name: "status and source filters pass through",
			target: "/backtest-runs?status=failed&strategyScriptId=88&tradingStrategyId=7&limit=5",
			expectedFilter: dto.BacktestRunListQueryDto{
				Status: "failed", StrategyScriptID: 88, TradingStrategyID: 7, Limit: 5,
			}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunRouterUnderTest(t)
			fixture.runs.runRepository.EXPECT().
				FindPageByOwner(gomock.Any(), signedInViewerID, testCase.expectedFilter).
				Return([]entities.BacktestRun{aSucceededBacktestRun(12, vo.BacktestRunSpotScript)}, int64(25), nil)

			response := fixture.send(http.MethodGet, testCase.target)

			require.Equal(t, http.StatusOK, response.Code)
			body := struct {
				Runs       []map[string]any `json:"runs"`
				TotalCount int              `json:"totalCount"`
			}{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, 25, body.TotalCount)
			require.Len(t, body.Runs, 1)
			assert.Equal(t, "succeeded", body.Runs[0]["status"])
			assert.Equal(t, 0.255, body.Runs[0]["totalReturnRate"])
			assert.Equal(t, 0.101, body.Runs[0]["maximumDrawdown"])
			assert.Equal(t, float64(4), body.Runs[0]["tradeCount"])
			assert.NotContains(t, body.Runs[0], "result")
		})
	}
}

func TestListBacktestRunsEndpointWithNoRunsAnswersAnEmptyList(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	fixture.runs.runRepository.EXPECT().FindPageByOwner(gomock.Any(), signedInViewerID, gomock.Any()).
		Return(nil, int64(0), nil)

	response := fixture.send(http.MethodGet, "/backtest-runs")

	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"runs":[],"totalCount":0}`, response.Body.String())
}

func TestCancelBacktestRunEndpoint(t *testing.T) {
	t.Run("a queued run of the viewer becomes cancelled", func(t *testing.T) {
		fixture := newBacktestRunRouterUnderTest(t)
		fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(20)).
			Return(aStoredBacktestRun(20, vo.BacktestRunQueued), true, nil)
		fixture.runs.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, backtestRun entities.BacktestRun) (bool, error) {
				assert.Equal(t, "cancelled", backtestRun.Status)
				assert.Equal(t, signedInViewerID, backtestRun.OwnerID)
				return true, nil
			})

		response := fixture.send(http.MethodPost, "/backtest-runs/20/cancel")

		require.Equal(t, http.StatusOK, response.Code)
		body := map[string]any{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "cancelled", body["status"])
		assert.Equal(t, "2026-08-30T00:00:00Z", body["finishedAt"])
	})

	t.Run("a run already ended is refused as such", func(t *testing.T) {
		fixture := newBacktestRunRouterUnderTest(t)
		fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(22)).
			Return(aSucceededBacktestRun(22, vo.BacktestRunSpotScript), true, nil)

		response := fixture.send(http.MethodPost, "/backtest-runs/22/cancel")

		require.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "已經結束")
	})

	t.Run("a run that ends between the read and the cancel is refused the same way", func(t *testing.T) {
		fixture := newBacktestRunRouterUnderTest(t)
		fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(21)).
			Return(aStoredBacktestRun(21, vo.BacktestRunRunning), true, nil)
		fixture.runs.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).Return(false, nil)

		response := fixture.send(http.MethodPost, "/backtest-runs/21/cancel")

		assert.Equal(t, http.StatusConflict, response.Code)
	})

	t.Run("somebody else's run is not found and left alone", func(t *testing.T) {
		fixture := newBacktestRunRouterUnderTest(t)
		fixture.clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()
		fixture.runs.runRepository.EXPECT().FindOneOwned(gomock.Any(), signedInViewerID, uint(25)).
			Return(entities.BacktestRun{}, false, nil)

		response := fixture.send(http.MethodPost, "/backtest-runs/25/cancel")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestBacktestRunEndpointsRefuseAnUnreadableIdentifier(t *testing.T) {
	for _, target := range []string{"/backtest-runs/abc", "/backtest-runs/0/cancel"} {
		t.Run(target, func(t *testing.T) {
			fixture := newBacktestRunRouterUnderTest(t)
			method := http.MethodGet
			if target == "/backtest-runs/0/cancel" {
				method = http.MethodPost
			}

			response := fixture.send(method, target)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestBacktestRunEndpointsWhenStorageFails(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	fixture.runs.runRepository.EXPECT().FindPageByOwner(gomock.Any(), signedInViewerID, gomock.Any()).
		Return(nil, int64(0), assert.AnError)

	response := fixture.send(http.MethodGet, "/backtest-runs")

	assert.Equal(t, http.StatusBadGateway, response.Code)
}

func TestBacktestRunEndpointsNeedASignedInCaller(t *testing.T) {
	fixture := newBacktestRunRouterUnderTest(t)
	for _, route := range []struct{ method, target string }{
		{http.MethodGet, "/backtest-runs"}, {http.MethodGet, "/backtest-runs/12"},
		{http.MethodPost, "/backtest-runs/12/cancel"},
	} {
		response := requestWithoutProof(fixture.engine, route.method, route.target, "")
		assert.Equal(t, http.StatusUnauthorized, response.Code, route.target)
	}
}

func jsonNumber(id uint) string {
	encoded, _ := json.Marshal(id)

	return string(encoded)
}
