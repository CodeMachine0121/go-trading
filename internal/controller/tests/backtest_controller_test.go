package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// backtestRouterNow pins the request time so "up to when" doesn't depend on when the suite runs.
var backtestRouterNow = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

var backtestRouterStart = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

const backtestBody = `{
	"symbol":"BTCUSDT",
	"aggregationInterval":"1h",
	"startTime":"2026-08-29T00:00:00Z",
	"endTime":"2026-08-29T04:00:00Z",
	"strategyScriptId":9,
	"initialCapital":"10000",
	"positionSizingMode":"allIn"
}`

const (
	// backtestRouterStrategyScriptID belongs to the signed-in viewer; the foreign one to somebody else.
	backtestRouterStrategyScriptID        = uint(9)
	backtestRouterForeignStrategyScriptID = uint(90)
)

type backtestRouterUnderTest struct {
	engine *gin.Engine
	runs   routerBacktestRuns
}

func newBacktestRouterUnderTest(t *testing.T) backtestRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestRouterStrategyScriptID).
		Return(entities.StrategyScript{
			ID: backtestRouterStrategyScriptID, OwnerID: signedInViewerID, Name: "均線", Script: "the script",
		}, nil).AnyTimes()
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestRouterForeignStrategyScriptID).
		Return(entities.StrategyScript{
			ID: backtestRouterForeignStrategyScriptID, OwnerID: signedInViewerID + 1, Script: "theirs",
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	backtestService := service.NewBacktestService(
		mocks.NewMockIKCandleRepository(mockController), mocks.NewMockIIndicatorScriptProxy(mockController),
		clockProxy, queryMaxResults, 0)
	runs := newRouterBacktestRuns(t, mockController, clockProxy, backtestService, nil)
	backtestController := controller.NewBacktestController(application.NewBacktestApplication(
		service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
		backtestService, nil, runs.backtestRunService))

	engine := gin.New()
	engine.POST("/backtests", doorOpenFor(t, signedInViewerID), backtestController.SubmitBacktest)

	return backtestRouterUnderTest{engine: engine, runs: runs}
}

func (fixture backtestRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/backtests", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// backtestBodyWith swaps one line of the well-formed body for another, keeping the rest valid.
func backtestBodyWith(replaced string, replacement string) string {
	return strings.Replace(backtestBody, replaced, replacement, 1)
}

func TestSubmitBacktestEndpoint(t *testing.T) {
	t.Run("a valid request is answered at once with a queued run and no report", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		saved := fixture.runs.expectQueued()

		response := fixture.post(backtestBody)

		require.Equal(t, http.StatusAccepted, response.Code)
		body := map[string]any{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, float64(queuedBacktestRunID), body["id"])
		assert.Equal(t, "spotScript", body["kind"])
		assert.Equal(t, "queued", body["status"])
		assert.Equal(t, float64(0), body["progressPercent"])
		assert.Equal(t, "BTCUSDT", body["symbol"])
		assert.Equal(t, "1h", body["interval"])
		assert.Equal(t, "2026-08-29T00:00:00Z", body["startTime"])
		assert.Equal(t, "2026-08-29T04:00:00Z", body["endTime"])
		assert.Equal(t, float64(backtestRouterStrategyScriptID), body["strategyScriptId"])
		assert.Equal(t, "均線", body["subjectName"])
		assert.Equal(t, "2026-08-30T00:00:00Z", body["submittedAt"])
		assert.Equal(t, "", body["failureReason"])
		for _, emptyUntilLater := range []string{
			"leverage", "tradingMode", "tradingStrategyId", "startedAt", "finishedAt",
			"totalReturnRate", "maximumDrawdown", "tradeCount",
		} {
			value, isPresent := body[emptyUntilLater]
			assert.True(t, isPresent, emptyUntilLater)
			assert.Nil(t, value, emptyUntilLater)
		}
		assert.NotContains(t, body, "result")
		assert.NotContains(t, body, "summary")
		assert.Equal(t, signedInViewerID, saved.OwnerID)
	})

	t.Run("saying spot out loud is queued exactly as saying nothing", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.runs.expectQueued()

		response := fixture.post(backtestBodyWith(`"positionSizingMode":"allIn"`,
			`"positionSizingMode":"allIn", "tradingMode":"spot"`))

		assert.Equal(t, http.StatusAccepted, response.Code)
	})
}

func TestSubmitBacktestEndpointRefusesBeforeQueueing(t *testing.T) {
	testCases := []struct {
		name            string
		body            string
		expectedStatus  int
		expectedField   string
		expectedMessage string
	}{
		{
			name:           "a body that is not readable",
			body:           `{"symbol":`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "a stretch that ends before it starts names the time range",
			body:           backtestBodyWith(`"endTime":"2026-08-29T04:00:00Z"`, `"endTime":"2026-08-28T00:00:00Z"`),
			expectedStatus: http.StatusBadRequest, expectedField: "timeRange",
		},
		{
			name:           "starting capital of zero names that field",
			body:           backtestBodyWith(`"initialCapital":"10000"`, `"initialCapital":"0"`),
			expectedStatus: http.StatusBadRequest, expectedField: "initialCapital", expectedMessage: "初始資金",
		},
		{
			name: "a sizing figure out of range names that figure",
			body: backtestBodyWith(`"positionSizingMode":"allIn"`,
				`"positionSizingMode":"percentage", "positionSizingValue":"0"`),
			expectedStatus: http.StatusBadRequest, expectedField: "positionSizingValue",
		},
		{
			name: "a knob nobody declared is refused by its name",
			body: backtestBodyWith(`"positionSizingMode":"allIn"`,
				`"positionSizingMode":"allIn", "parameterValues":[{"name":"不存在的旋鈕","value":3}]`),
			expectedStatus: http.StatusBadRequest, expectedMessage: "不存在的旋鈕",
		},
		{
			name: "any set of rules other than spot names the input at fault",
			body: backtestBodyWith(`"positionSizingMode":"allIn"`,
				`"positionSizingMode":"allIn", "tradingMode":"longShort"`),
			expectedStatus: http.StatusBadRequest, expectedField: "tradingMode", expectedMessage: "只重演現貨",
		},
		{
			name: "asking to borrow names its own box",
			body: backtestBodyWith(`"positionSizingMode":"allIn"`,
				`"positionSizingMode":"allIn", "leverage":"3"`),
			expectedStatus: http.StatusBadRequest, expectedField: "leverage", expectedMessage: "沒有人借錢給你",
		},
		{
			name: "a rate for being closed out is answered, not ignored",
			body: backtestBodyWith(`"positionSizingMode":"allIn"`,
				`"positionSizingMode":"allIn", "maintenanceMarginRate":"0.5"`),
			expectedStatus: http.StatusBadRequest, expectedField: "leverage", expectedMessage: "沒有維持保證金率",
		},
		{
			name:           "a script that is somebody else's is not found",
			body:           backtestBodyWith(`"strategyScriptId":9`, `"strategyScriptId":90`),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRouterUnderTest(t)
			// Even at the owner limit the request's own fault is what is reported.
			fixture.runs.ownerHasActive(routerBacktestRunOwnerActiveLimit)

			response := fixture.post(testCase.body)

			require.Equal(t, testCase.expectedStatus, response.Code, response.Body.String())
			body := struct {
				Message string `json:"message"`
				Field   string `json:"field"`
			}{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, testCase.expectedField, body.Field)
			assert.Contains(t, body.Message, testCase.expectedMessage)
		})
	}
}

func TestSubmitBacktestEndpointAtTheOwnerLimit(t *testing.T) {
	fixture := newBacktestRouterUnderTest(t)
	fixture.runs.ownerHasActive(routerBacktestRunOwnerActiveLimit)

	response := fixture.post(backtestBody)

	require.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.JSONEq(t, `{"message":"你同時最多 3 筆回測在排隊或進行中，等其中一筆結束再送","ownerActiveLimit":3}`,
		response.Body.String())
}

func TestSubmitBacktestEndpointWhenTheQueueCannotBeRead(t *testing.T) {
	fixture := newBacktestRouterUnderTest(t)
	fixture.runs.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), signedInViewerID).
		Return(0, assert.AnError)

	response := fixture.post(backtestBody)

	assert.Equal(t, http.StatusBadGateway, response.Code)
}

func TestSubmitBacktestEndpointNeedsASignedInCaller(t *testing.T) {
	fixture := newBacktestRouterUnderTest(t)

	response := requestWithoutProof(fixture.engine, http.MethodPost, "/backtests", backtestBody)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
