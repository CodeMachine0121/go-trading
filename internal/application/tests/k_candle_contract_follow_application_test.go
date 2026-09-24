package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Shutting the contract live follow down closes every contract viewer's updates and
// turns later ones away — the one thing the entry point asks of it on the way down.
func TestStoppingTheContractLiveFollowEndsEveryViewer(t *testing.T) {
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)).AnyTimes()
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).
		Return(make(chan vo.LiveKCandleVo), nil).AnyTimes()
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil).AnyTimes()

	followApplication := application.NewKCandleContractFollowApplication(
		service.NewKCandleContractFollowService(
			liveMarketDataProxy, contractTradingSymbolRepository, clockProxy,
			time.Nanosecond, time.Hour, time.Millisecond))

	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, watchError := followApplication.WatchKCandleContracts(viewer, "BTCUSDT")
	require.NoError(t, watchError)

	followApplication.Stop()

	_, stillOpen := <-updates
	assert.False(t, stillOpen)
	_, lateError := followApplication.WatchKCandleContracts(context.Background(), "BTCUSDT")
	assert.ErrorIs(t, lateError, service.ErrKCandleFollowStopped)
}
