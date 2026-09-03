package controller_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var followingOpenTime = time.Date(2026, 9, 3, 9, 5, 0, 0, time.UTC)

// streamRecorder collects a response that is still being written.
//
// The recorder from the standard library is not built to be read while a handler
// is still writing to it, and a stream is by definition read while it is still
// being written — so reading one with that recorder is a race, and the race
// detector says so. This one guards the body, which is the whole difference.
type streamRecorder struct {
	responseHeader http.Header
	mutex          sync.Mutex
	body           strings.Builder
	statusCode     int
}

func newStreamRecorder() *streamRecorder {
	return &streamRecorder{responseHeader: make(http.Header), statusCode: http.StatusOK}
}

func (streamRecorder *streamRecorder) Header() http.Header {
	return streamRecorder.responseHeader
}

func (streamRecorder *streamRecorder) WriteHeader(statusCode int) {
	streamRecorder.mutex.Lock()
	defer streamRecorder.mutex.Unlock()

	streamRecorder.statusCode = statusCode
}

func (streamRecorder *streamRecorder) Write(body []byte) (int, error) {
	streamRecorder.mutex.Lock()
	defer streamRecorder.mutex.Unlock()

	return streamRecorder.body.Write(body)
}

// Flush is required rather than optional: the handler flushes after every event,
// and the framework reaches for the flusher without asking whether there is one.
func (streamRecorder *streamRecorder) Flush() {}

func (streamRecorder *streamRecorder) written() string {
	streamRecorder.mutex.Lock()
	defer streamRecorder.mutex.Unlock()

	return streamRecorder.body.String()
}


// followRouterUnderTest mounts the live route over a follow service whose feed the
// test hands out, so what a viewer receives can be read off the response body.
type followRouterUnderTest struct {
	engine       *gin.Engine
	liveKCandles chan vo.LiveKCandleVo
	stop         func()
}

func newFollowRouterUnderTest(t *testing.T, followError error) followRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	// A candle that closes is stored on its way past; storing is pinned by the
	// follow's own tests, so here it only has to be allowed to happen.
	kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)

	readings := 0
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		readings++

		return followingOpenTime.Add(time.Duration(readings) * time.Minute)
	}).AnyTimes()

	liveKCandles := make(chan vo.LiveKCandleVo, 4)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, string) (<-chan vo.LiveKCandleVo, error) {
			if followError != nil {
				return nil, followError
			}

			return liveKCandles, nil
		}).AnyTimes()

	kCandleFollowService := service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, clockProxy,
		time.Nanosecond, time.Hour, 10*time.Millisecond,
	)
	t.Cleanup(kCandleFollowService.Stop)

	engine := gin.New()
	engine.GET("/k-candles/live", controller.NewKCandleFollowController(
		application.NewKCandleFollowApplication(kCandleFollowService)).WatchKCandles)

	return followRouterUnderTest{
		engine:       engine,
		liveKCandles: liveKCandles,
		stop:         kCandleFollowService.Stop,
	}
}

// Without a symbol there is no market to follow, and guessing one would be worse
// than saying so.
func TestFollowingWithoutASymbolIsRefused(t *testing.T) {
	router := newFollowRouterUnderTest(t, nil)

	recorder := httptest.NewRecorder()
	router.engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/k-candles/live", nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "請指定交易標的")
}

// A viewer arriving after shutdown has begun is turned away rather than left on a
// connection nothing will ever feed.
func TestFollowingAfterShutdownIsTurnedAway(t *testing.T) {
	router := newFollowRouterUnderTest(t, nil)
	router.stop()

	recorder := httptest.NewRecorder()
	router.engine.ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/k-candles/live?symbol=BTCUSDT", nil))

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

// What the viewer receives is one event per update, carrying the candle and which
// of the three states it is in.
func TestEachUpdateIsWrittenAsOneEvent(t *testing.T) {
	router := newFollowRouterUnderTest(t, nil)

	requestContext, endTheRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/k-candles/live?symbol=BTCUSDT", nil).
		WithContext(requestContext)
	recorder := newStreamRecorder()

	router.liveKCandles <- vo.LiveKCandleVo{
		Symbol:   "BTCUSDT",
		OpenTime: followingOpenTime,
		Close:    decimal.RequireFromString("118.25"),
		Closed:   true,
	}

	served := make(chan struct{})
	go func() {
		router.engine.ServeHTTP(recorder, request)
		close(served)
	}()

	assert.Eventually(t, func() bool { return strings.Contains(recorder.written(), "data:") },
		2*time.Second, 10*time.Millisecond)
	endTheRequest()

	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("請求結束了，串流卻沒有跟著收掉")
	}

	body := recorder.written()
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Contains(t, body, `"status":"closed"`)
	assert.Contains(t, body, `"symbol":"BTCUSDT"`)
	assert.Contains(t, body, `"close":"118.25"`)
	assert.True(t, strings.HasSuffix(body, "\n\n"), "每一則更新自成一個事件")
}

// A viewer walking away must end the request rather than hold a connection open
// against a market nobody is watching.
func TestTheRequestEndsWhenTheViewerLeaves(t *testing.T) {
	router := newFollowRouterUnderTest(t, nil)

	requestContext, viewerLeaves := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/k-candles/live?symbol=BTCUSDT", nil).
		WithContext(requestContext)

	served := make(chan struct{})
	go func() {
		router.engine.ServeHTTP(httptest.NewRecorder(), request)
		close(served)
	}()

	viewerLeaves()

	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("觀看者走了，請求卻沒有結束")
	}
}

// A source that will not answer is not a reason to refuse the viewer: the follow
// keeps retrying and tells them it is stalled, which is a live connection saying so.
func TestASourceThatRefusesStillOpensTheStream(t *testing.T) {
	router := newFollowRouterUnderTest(t, errors.New("行情來源拒絕連線"))

	requestContext, endTheRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/k-candles/live?symbol=BTCUSDT", nil).
		WithContext(requestContext)
	recorder := newStreamRecorder()

	served := make(chan struct{})
	go func() {
		router.engine.ServeHTTP(recorder, request)
		close(served)
	}()

	require.Eventually(t, func() bool { return strings.Contains(recorder.written(), "stalled") },
		2*time.Second, 10*time.Millisecond, "跟不動時觀看者該收到明說停止的更新")
	endTheRequest()
	<-served
}
