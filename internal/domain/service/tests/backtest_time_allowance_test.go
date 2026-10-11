package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
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

const replayTimeAllowanceUnderTest = 20 * time.Millisecond

// aScriptOutlastingItsAllowance blocks until the caller stops waiting, like a script over a very long stretch.
func aScriptOutlastingItsAllowance[Input any](
	executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
	_ []Input, _ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
) ([]map[string]vo.IndicatorValueVo, error) {
	<-executionContext.Done()

	return nil, errors.New("stopped because the request ended")
}

func spotStoredCandles() []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, 2)
	for hour := range 2 {
		price := decimal.NewFromInt(100)
		kCandles = append(kCandles, entities.KCandle{
			Symbol: "BTCUSDT", OpenTime: contractBacktestServiceStart.Add(time.Duration(hour) * time.Hour),
			Open: price, High: price, Low: price, Close: price,
		})
	}

	return kCandles
}

func TestSpotReplayStopsWhenItsTimeAllowanceRunsOut(t *testing.T) {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(spotStoredCandles(), nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(aScriptOutlastingItsAllowance[vo.KCandleVo]).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(contractBacktestServiceStart.Add(24 * time.Hour)).AnyTimes()
	backtestService := service.NewBacktestService(
		kCandleRepository, indicatorScriptProxy, clockProxy, 1000, 0)

	t.Run("replaying a script", func(t *testing.T) {
		_, err := backtestService.RunBacktest(context.Background(), dto.BacktestRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1h",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			Script: "the script", InitialCapital: decimal.NewFromInt(10000),
			TimeAllowance: replayTimeAllowanceUnderTest,
		}, nil)

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
		assert.Contains(t, err.Error(), "0.02 秒")
	})

	t.Run("replaying a trading strategy", func(t *testing.T) {
		_, err := backtestService.RunTradingStrategyBacktest(context.Background(), dto.TradingStrategyBacktestRequestDto{
			Symbol:    "BTCUSDT",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			SignalSources: []dto.ResolvedSignalSourceDto{
				{Label: "A", AggregationInterval: "1h", Script: "the script"},
			},
			BuyCondition:   dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "buy"},
			SellCondition:  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "sell"},
			InitialCapital: decimal.NewFromInt(10000),
			TimeAllowance:  replayTimeAllowanceUnderTest,
		}, nil)

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})

	t.Run("whoever asked giving up is not the allowance running out", func(t *testing.T) {
		askerContext, giveUp := context.WithCancel(context.Background())
		giveUp()

		_, err := backtestService.RunBacktest(askerContext, dto.BacktestRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1h",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			Script: "the script", InitialCapital: decimal.NewFromInt(10000),
			TimeAllowance: replayTimeAllowanceUnderTest,
		}, nil)

		require.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})
}

func TestContractReplayStopsWhenItsTimeAllowanceRunsOut(t *testing.T) {
	stopsSlowly := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(aScriptOutlastingItsAllowance[vo.ContractKCandleVo])
	}

	t.Run("replaying a script", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, stopsSlowly)
		requestDto := contractBacktestServiceRequest()
		requestDto.TimeAllowance = replayTimeAllowanceUnderTest

		_, err := contractBacktestService.RunContractBacktest(context.Background(), requestDto, nil)

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
		assert.Contains(t, err.Error(), "0.02 秒")
	})

	t.Run("replaying a trading strategy", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, stopsSlowly)
		requestDto := contractTradingStrategyBacktestServiceRequest()
		requestDto.TimeAllowance = replayTimeAllowanceUnderTest

		_, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})
}

func TestContractReplayAllowanceCoversEverySourceTogether(t *testing.T) {
	// Each source takes 15ms on its own — within 20ms alone, beyond it together.
	takesFifteenMilliseconds := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
				contractKCandles []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
				_ domaininterface.IReplayProgressSink,
			) ([]map[string]vo.IndicatorValueVo, error) {
				select {
				case <-executionContext.Done():
					return nil, errors.New("stopped because the request ended")
				case <-time.After(15 * time.Millisecond):
				}

				perBar := make([]map[string]vo.IndicatorValueVo, 0, len(contractKCandles))
				for range contractKCandles {
					perBar = append(perBar, map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalHold}})
				}

				return perBar, nil
			}).Times(2)
	}
	requestDto := contractTradingStrategyBacktestServiceRequest()
	requestDto.SignalSources = append(requestDto.SignalSources, dto.ResolvedSignalSourceDto{
		Label: "B", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle",
	})

	requestDto.TimeAllowance = replayTimeAllowanceUnderTest
	contractBacktestService := newContractBacktestServiceUnderTest(t, takesFifteenMilliseconds)

	_, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

	require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
}

func TestContractSplitReplayRunsTheScriptOnceOverTheWholeHistory(t *testing.T) {
	onceOverEveryBar := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Len(2), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil).Times(1)
	}
	requestDto := contractBacktestServiceRequest()
	requestDto.ValidationStartTime = contractBacktestServiceStart.Add(time.Hour)

	resultDto, err := newContractBacktestServiceUnderTest(t, onceOverEveryBar).
		RunContractBacktest(context.Background(), requestDto, nil)

	require.NoError(t, err)
	require.NotNil(t, resultDto.Validation)
	assert.Equal(t, 1, resultDto.Validation.UsedCandleCount)
}

func TestSpotReplayAllowanceCoversReadingTheMarket(t *testing.T) {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, _ domains.KCandleQueryDomain, _ int) ([]entities.KCandle, error) {
			<-executionContext.Done()

			return nil, executionContext.Err()
		})
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(contractBacktestServiceStart.Add(24 * time.Hour)).AnyTimes()
	backtestService := service.NewBacktestService(
		kCandleRepository, mocks.NewMockIIndicatorScriptProxy(controller), clockProxy, 1000, 0)

	_, err := backtestService.RunBacktest(context.Background(), dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
		Script: "the script", InitialCapital: decimal.NewFromInt(10000),
		TimeAllowance: replayTimeAllowanceUnderTest,
	}, nil)

	require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
}

func TestContractReplayAllowanceCoversReadingTheMarket(t *testing.T) {
	readsSlowly := func(boundaries contractBacktestServiceMocks) {
		boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(executionContext context.Context, _ domains.KCandleQueryDomain, _ int) ([]entities.KCandleContract, error) {
				<-executionContext.Done()

				return nil, executionContext.Err()
			})
	}

	requestDto := contractBacktestServiceRequest()
	requestDto.TimeAllowance = replayTimeAllowanceUnderTest

	_, err := newContractBacktestServiceUnderTest(t, readsSlowly).RunContractBacktest(context.Background(), requestDto, nil)

	require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
}

// neverGivenACompartment waits out the allowance for a compartment slot, as the script proxy does when every slot stays taken.
func neverGivenACompartment[Input any](
	executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
	_ []Input, _ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
) ([]map[string]vo.IndicatorValueVo, error) {
	<-executionContext.Done()

	return nil, fmt.Errorf("%w: every slot stayed taken", domains.ErrIndicatorScriptCompartmentsBusy)
}

func TestReplayWaitingForACompartmentIsBlamedOnWhoeverStoppedTheWait(t *testing.T) {
	spotReplay := func(t *testing.T, executionContext context.Context) error {
		controller := gomock.NewController(t)
		kCandleRepository := mocks.NewMockIKCandleRepository(controller)
		kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(spotStoredCandles(), nil)
		indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
		indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(neverGivenACompartment[vo.KCandleVo])
		clockProxy := mocks.NewMockIClockProxy(controller)
		clockProxy.EXPECT().Now().Return(contractBacktestServiceStart.Add(24 * time.Hour)).AnyTimes()

		_, err := service.NewBacktestService(
			kCandleRepository, indicatorScriptProxy, clockProxy, 1000, 0,
		).RunBacktest(executionContext, dto.BacktestRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1h",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			Script: "the script", InitialCapital: decimal.NewFromInt(10000),
			TimeAllowance: replayTimeAllowanceUnderTest,
		}, nil)
		return err
	}
	contractReplay := func(t *testing.T, executionContext context.Context) error {
		contractBacktestService := newContractBacktestServiceUnderTest(t,
			func(boundaries contractBacktestServiceMocks) {
				boundaries.contractIndicatorScriptProxy.EXPECT().
					ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(neverGivenACompartment[vo.ContractKCandleVo])
			})
		requestDto := contractBacktestServiceRequest()
		requestDto.TimeAllowance = replayTimeAllowanceUnderTest

		_, err := contractBacktestService.RunContractBacktest(executionContext, requestDto, nil)
		return err
	}
	callerGivingUpFirst := func(t *testing.T) context.Context {
		callerContext, giveUp := context.WithTimeout(context.Background(), replayTimeAllowanceUnderTest/4)
		t.Cleanup(giveUp)
		return callerContext
	}
	callerWaiting := func(*testing.T) context.Context { return context.Background() }

	testCases := []struct {
		name          string
		replay        func(t *testing.T, executionContext context.Context) error
		caller        func(t *testing.T) context.Context
		expectedError error
		unexpected    error
	}{
		{name: "a spot replay that waits out its allowance", replay: spotReplay, caller: callerWaiting,
			expectedError: domains.ErrBacktestTimeAllowanceSpent, unexpected: domains.ErrIndicatorScriptCompartmentsBusy},
		{name: "a contract replay that waits out its allowance", replay: contractReplay, caller: callerWaiting,
			expectedError: domains.ErrBacktestTimeAllowanceSpent, unexpected: domains.ErrIndicatorScriptCompartmentsBusy},
		{name: "a spot replay whose caller gives up waiting", replay: spotReplay, caller: callerGivingUpFirst,
			expectedError: domains.ErrIndicatorScriptCompartmentsBusy, unexpected: domains.ErrBacktestTimeAllowanceSpent},
		{name: "a contract replay whose caller gives up waiting", replay: contractReplay, caller: callerGivingUpFirst,
			expectedError: domains.ErrIndicatorScriptCompartmentsBusy, unexpected: domains.ErrBacktestTimeAllowanceSpent},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.replay(t, testCase.caller(t))

			require.ErrorIs(t, err, testCase.expectedError)
			assert.NotErrorIs(t, err, testCase.unexpected)
		})
	}
}
