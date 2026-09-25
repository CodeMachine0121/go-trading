package middlewares_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamRouteUnderTest holds streams on one route until released, and probes the shared cap on another that returns at once.
type streamRouteUnderTest struct {
	engine       *gin.Engine
	streamsOpen  *sync.WaitGroup
	releaseAll   chan struct{}
	streamsEnded *sync.WaitGroup
}

func newStreamRouteUnderTest(t *testing.T) streamRouteUnderTest {
	gin.SetMode(gin.TestMode)
	liveStreamLimit := middlewares.NewLiveStreamLimitMiddleware(newRequestAdmissionApplication(t)).Handle
	streamsOpen := &sync.WaitGroup{}
	releaseAll := make(chan struct{})
	engine := gin.New()
	engine.GET("/k-candles/live", liveStreamLimit, func(ginContext *gin.Context) {
		streamsOpen.Done()
		<-releaseAll
		ginContext.Status(http.StatusOK)
	})
	engine.GET("/contract-k-candles/live", liveStreamLimit, func(ginContext *gin.Context) {
		ginContext.Status(http.StatusOK)
	})

	return streamRouteUnderTest{
		engine: engine, streamsOpen: streamsOpen, releaseAll: releaseAll, streamsEnded: &sync.WaitGroup{},
	}
}

func (routeUnderTest streamRouteUnderTest) openAndHold(count int) {
	routeUnderTest.streamsOpen.Add(count)
	for range count {
		routeUnderTest.streamsEnded.Go(func() {
			routeUnderTest.engine.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/k-candles/live", nil))
		})
	}
	routeUnderTest.streamsOpen.Wait()
}

func (routeUnderTest streamRouteUnderTest) releaseHeld() {
	close(routeUnderTest.releaseAll)
	routeUnderTest.streamsEnded.Wait()
}

func TestLiveStreamLimitRefusesPastTheCapAndFreesAPlaceWhenAStreamEnds(t *testing.T) {
	testCases := []struct {
		name           string
		heldStreams    int
		releaseFirst   bool
		expectedStatus int
	}{
		{name: "the twentieth stream opens", heldStreams: 19, expectedStatus: http.StatusOK},
		{name: "the twenty-first stream is refused", heldStreams: 20, expectedStatus: http.StatusTooManyRequests},
		{name: "places free once streams end", heldStreams: 20, releaseFirst: true,
			expectedStatus: http.StatusOK},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			routeUnderTest := newStreamRouteUnderTest(t)
			routeUnderTest.openAndHold(testCase.heldStreams)
			if testCase.releaseFirst {
				routeUnderTest.releaseHeld()
			}

			recorder := httptest.NewRecorder()
			routeUnderTest.engine.ServeHTTP(recorder,
				httptest.NewRequest(http.MethodGet, "/contract-k-candles/live", nil))
			if !testCase.releaseFirst {
				routeUnderTest.releaseHeld()
			}

			assert.Equal(t, testCase.expectedStatus, recorder.Code)
			if testCase.expectedStatus == http.StatusTooManyRequests {
				var body map[string]string
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Equal(t, "同時開著的即時跟盤已達上限，請先關掉一些再開", body["message"])
			}
		})
	}
}
