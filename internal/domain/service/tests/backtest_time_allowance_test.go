package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

// aScriptOutlastingItsAllowance runs until whoever asked stops waiting, the way a
// script over a very long stretch would.
func aScriptOutlastingItsAllowance[Input any](
	executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
	_ []Input, _ domains.StrategyScriptParametersDomain,
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
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(aScriptOutlastingItsAllowance[vo.KCandleVo]).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(contractBacktestServiceStart.Add(24 * time.Hour)).AnyTimes()
	backtestService := service.NewBacktestService(
		kCandleRepository, indicatorScriptProxy, clockProxy, 1000, replayTimeAllowanceUnderTest)

	t.Run("replaying a script", func(t *testing.T) {
		_, err := backtestService.RunBacktest(context.Background(), dto.BacktestRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1h",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			Script: "the script", InitialCapital: decimal.NewFromInt(10000),
		})

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
		})

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})

	t.Run("whoever asked giving up is not the allowance running out", func(t *testing.T) {
		askerContext, giveUp := context.WithCancel(context.Background())
		giveUp()

		_, err := backtestService.RunBacktest(askerContext, dto.BacktestRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1h",
			StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
			Script: "the script", InitialCapital: decimal.NewFromInt(10000),
		})

		require.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})
}

func TestContractReplayStopsWhenItsTimeAllowanceRunsOut(t *testing.T) {
	stopsSlowly := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(aScriptOutlastingItsAllowance[vo.ContractKCandleVo])
	}

	t.Run("replaying a script", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceWithin(t, replayTimeAllowanceUnderTest, stopsSlowly)

		_, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest())

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
		assert.Contains(t, err.Error(), "0.02 秒")
	})

	t.Run("replaying a trading strategy", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceWithin(t, replayTimeAllowanceUnderTest, stopsSlowly)

		_, err := contractBacktestService.RunContractTradingStrategyBacktest(
			context.Background(), contractTradingStrategyBacktestServiceRequest())

		require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
	})
}

func TestContractReplayAllowanceCoversEverySourceTogether(t *testing.T) {
	// Each source takes 15ms on its own — within 20ms alone, beyond it together.
	takesFifteenMilliseconds := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
				contractKCandles []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
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

	contractBacktestService := newContractBacktestServiceWithin(t, replayTimeAllowanceUnderTest, takesFifteenMilliseconds)

	_, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto)

	require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
}

func TestContractSplitReplayRunsTheScriptOnceOverTheWholeHistory(t *testing.T) {
	onceOverEveryBar := func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Len(2), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil).Times(1)
	}
	requestDto := contractBacktestServiceRequest()
	requestDto.ValidationStartTime = contractBacktestServiceStart.Add(time.Hour)

	resultDto, err := newContractBacktestServiceWithin(t, time.Minute, onceOverEveryBar).
		RunContractBacktest(context.Background(), requestDto)

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
		kCandleRepository, mocks.NewMockIIndicatorScriptProxy(controller), clockProxy, 1000, replayTimeAllowanceUnderTest)

	_, err := backtestService.RunBacktest(context.Background(), dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
		Script: "the script", InitialCapital: decimal.NewFromInt(10000),
	})

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

	_, err := newContractBacktestServiceWithin(t, replayTimeAllowanceUnderTest, readsSlowly).
		RunContractBacktest(context.Background(), contractBacktestServiceRequest())

	require.ErrorIs(t, err, domains.ErrBacktestTimeAllowanceSpent)
}
