package controller_test

import (
	"context"
	"errors"
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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// contractFollowRouterUnderTest mounts the contract live route over a contract follow
// whose feed the test hands out. BTCUSDT is on the contract watchlist, DOGEUSDT is known
// but not followed, and anything else is unknown. The spot route is mounted too, over a
// spot follow that must never be reached from here.
type contractFollowRouterUnderTest struct {
	engine       *gin.Engine
	liveKCandles chan vo.LiveKCandleVo
	stop         func()
}

func newContractFollowRouterUnderTest(t *testing.T, repositoryError error) contractFollowRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	clockProxy := mocks.NewMockIClockProxy(mockController)
	readings := 0
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		readings++

		return followingOpenTime.Add(time.Duration(readings) * time.Minute)
	}).AnyTimes()

	liveKCandles := make(chan vo.LiveKCandleVo, 4)
	contractLiveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	contractLiveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).
		Return(liveKCandles, nil).AnyTimes()

	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.ContractTradingSymbol, bool, error) {
			if repositoryError != nil {
				return entities.ContractTradingSymbol{}, false, repositoryError
			}

			switch symbol {
			case "BTCUSDT":
				return entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true}, true, nil
			case "DOGEUSDT":
				return entities.ContractTradingSymbol{Symbol: symbol}, true, nil
			}

			return entities.ContractTradingSymbol{}, false, nil
		}).AnyTimes()

	contractFollowService := service.NewKCandleContractFollowService(
		contractLiveMarketDataProxy, contractTradingSymbolRepository, clockProxy,
		time.Nanosecond, time.Hour, 10*time.Millisecond)
	t.Cleanup(contractFollowService.Stop)

	// The spot follow's source and registry have no expectations: a contract viewer
	// reaching either would fail the test.
	spotFollowService := service.NewKCandleFollowService(
		mocks.NewMockILiveMarketDataProxy(mockController), mocks.NewMockIKCandleRepository(mockController),
		mocks.NewMockITradingSymbolRepository(mockController), clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}), time.Nanosecond, time.Hour, 10*time.Millisecond)
	t.Cleanup(spotFollowService.Stop)

	followController := controller.NewKCandleFollowController(
		application.NewKCandleFollowApplication(spotFollowService),
		application.NewKCandleContractFollowApplication(contractFollowService))
	engine := gin.New()
	engine.GET("/k-candles/live", followController.WatchKCandles)
	engine.GET("/contract-k-candles/live", followController.WatchKCandleContracts)

	return contractFollowRouterUnderTest{
		engine: engine, liveKCandles: liveKCandles, stop: contractFollowService.Stop,
	}
}

func TestWatchingAContractIsRefusedWithTheStatusItsReasonCallsFor(t *testing.T) {
	testCases := []struct {
		name             string
		target           string
		repositoryError  error
		stopFirst        bool
		expectedCode     int
		expectedFragment string
	}{
		{name: "no contract named", target: "/contract-k-candles/live",
			expectedCode: http.StatusBadRequest, expectedFragment: "請指定合約標的"},
		{name: "a contract the system has never heard of", target: "/contract-k-candles/live?symbol=NOPEUSDT",
			expectedCode: http.StatusNotFound, expectedFragment: "NOPEUSDT"},
		{name: "a known contract that is not followed", target: "/contract-k-candles/live?symbol=DOGEUSDT",
			expectedCode: http.StatusConflict, expectedFragment: "請先把它加進合約追蹤名單"},
		{name: "a contract code that cannot be one", target: "/contract-k-candles/live?symbol=%20",
			expectedCode: http.StatusBadRequest},
		{name: "the contract's entry cannot be read", target: "/contract-k-candles/live?symbol=BTCUSDT",
			repositoryError: errors.New("the database went away"), expectedCode: http.StatusServiceUnavailable},
		{name: "arriving after shutdown", target: "/contract-k-candles/live?symbol=BTCUSDT",
			stopFirst: true, expectedCode: http.StatusServiceUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			router := newContractFollowRouterUnderTest(t, testCase.repositoryError)
			if testCase.stopFirst {
				router.stop()
			}

			recorder := httptest.NewRecorder()
			router.engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, testCase.target, nil))

			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Contains(t, recorder.Body.String(), testCase.expectedFragment)
		})
	}
}

// A contract viewer receives one event per update, in the same shape as a spot one.
func TestAContractUpdateIsWrittenAsOneEvent(t *testing.T) {
	router := newContractFollowRouterUnderTest(t, nil)

	requestContext, endTheRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/contract-k-candles/live?symbol=BTCUSDT", nil).
		WithContext(requestContext)
	recorder := newStreamRecorder()

	router.liveKCandles <- vo.LiveKCandleVo{
		Symbol: "BTCUSDT", OpenTime: followingOpenTime,
		Close: decimal.RequireFromString("64000.5"), Closed: true,
	}

	served := make(chan struct{})
	go func() {
		router.engine.ServeHTTP(recorder, request)
		close(served)
	}()

	require.Eventually(t, func() bool { return strings.Contains(recorder.written(), "data:") },
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
	assert.Contains(t, body, `"close":"64000.5"`)
	assert.True(t, strings.HasSuffix(body, "\n\n"), "每一則更新自成一個事件")
}
