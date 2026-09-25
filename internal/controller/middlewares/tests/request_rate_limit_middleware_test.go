package middlewares_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newRequestAdmissionApplication uses the shipped defaults with a frozen clock, so nothing refills mid-test.
func newRequestAdmissionApplication(t *testing.T) *application.RequestAdmissionApplication {
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)).AnyTimes()

	return application.NewRequestAdmissionApplication(service.NewRequestAdmissionService(
		mocks.NewMockIAccessTokenProxy(mockController),
		clockProxy,
		vo.RequestBudgetVo{RequestsPerMinute: 600, Burst: 120},
		vo.RequestBudgetVo{RequestsPerMinute: 10, Burst: 10},
		vo.LiveStreamCapacityVo{PerRequester: 20, Total: 1000},
	))
}

type throttledRouteUnderTest struct {
	engine          *gin.Engine
	handlerRunCount *int
}

func newThrottledRouteUnderTest(t *testing.T) throttledRouteUnderTest {
	gin.SetMode(gin.TestMode)
	requestRateLimitMiddleware := middlewares.NewRequestRateLimitMiddleware(newRequestAdmissionApplication(t))

	handlerRunCount := 0
	countingHandler := func(ginContext *gin.Context) {
		handlerRunCount++
		ginContext.Status(http.StatusOK)
	}
	engine := gin.New()
	engine.Use(requestRateLimitMiddleware.Handle)
	engine.GET("/k-candles", countingHandler)
	engine.POST("/sessions", requestRateLimitMiddleware.HandleCredentialRequest, countingHandler)

	return throttledRouteUnderTest{engine: engine, handlerRunCount: &handlerRunCount}
}

func TestRequestRateLimitRefusesASpentAllowanceWithTheWait(t *testing.T) {
	testCases := []struct {
		name               string
		method             string
		path               string
		earlierRequests    int
		expectedStatus     int
		expectedRetryAfter string
		expectedMessage    string
	}{
		{name: "within the burst", method: http.MethodGet, path: "/k-candles", earlierRequests: 119,
			expectedStatus: http.StatusOK},
		{name: "past the burst", method: http.MethodGet, path: "/k-candles", earlierRequests: 120,
			expectedStatus: http.StatusTooManyRequests, expectedRetryAfter: "1",
			expectedMessage: "請求太頻繁，請 1 秒後再試"},
		{name: "the tenth sign-in", method: http.MethodPost, path: "/sessions", earlierRequests: 9,
			expectedStatus: http.StatusOK},
		{name: "the eleventh sign-in", method: http.MethodPost, path: "/sessions", earlierRequests: 10,
			expectedStatus: http.StatusTooManyRequests, expectedRetryAfter: "6",
			expectedMessage: "請求太頻繁，請 6 秒後再試"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			routeUnderTest := newThrottledRouteUnderTest(t)
			for range testCase.earlierRequests {
				routeUnderTest.engine.ServeHTTP(httptest.NewRecorder(),
					httptest.NewRequest(testCase.method, testCase.path, nil))
			}
			handlerRunsBefore := *routeUnderTest.handlerRunCount

			recorder := httptest.NewRecorder()
			routeUnderTest.engine.ServeHTTP(recorder, httptest.NewRequest(testCase.method, testCase.path, nil))

			assert.Equal(t, testCase.expectedStatus, recorder.Code)
			assert.Equal(t, testCase.expectedRetryAfter, recorder.Header().Get("Retry-After"))
			if testCase.expectedStatus == http.StatusTooManyRequests {
				assert.Equal(t, handlerRunsBefore, *routeUnderTest.handlerRunCount, "the handler ran anyway")
				var body map[string]string
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Equal(t, testCase.expectedMessage, body["message"])
			}
		})
	}
}
