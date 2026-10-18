package service_test

import (
	"context"
	"errors"
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

var contractBacktestServiceStart = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

var errStorageUnavailable = errors.New("storage unavailable")

// contractBacktestServiceMocks answer normally unless a case makes one fail.
type contractBacktestServiceMocks struct {
	kCandleContractRepository    *mocks.MockIKCandleContractRepository
	settlementRepository         *mocks.MockIContractFundingRateSettlementRepository
	statisticRepository          *mocks.MockIContractPositionStatisticRepository
	tradingSymbolRepository      *mocks.MockIContractTradingSymbolRepository
	tierRepository               *mocks.MockIContractMaintenanceMarginTierRepository
	contractIndicatorScriptProxy *mocks.MockIContractIndicatorScriptProxy
	tradeHistoryProxy            *mocks.MockIContractTradeHistoryProxy
}

type contractBacktestServiceFailure struct {
	name     string
	failWith func(contractBacktestServiceMocks)
}

func newContractBacktestServiceUnderTest(
	t *testing.T, failure func(contractBacktestServiceMocks),
) *service.ContractBacktestService {
	return newContractBacktestServiceWarmingUp(t, failure, 0)
}

// newContractBacktestServiceWarmingUp adds that many bars of warm-up to every replay that names none, as the server does.
func newContractBacktestServiceWarmingUp(
	t *testing.T, failure func(contractBacktestServiceMocks), defaultWarmupBarCount int,
) *service.ContractBacktestService {
	controller := gomock.NewController(t)
	boundaries := contractBacktestServiceMocks{
		kCandleContractRepository:    mocks.NewMockIKCandleContractRepository(controller),
		settlementRepository:         mocks.NewMockIContractFundingRateSettlementRepository(controller),
		statisticRepository:          mocks.NewMockIContractPositionStatisticRepository(controller),
		tradingSymbolRepository:      mocks.NewMockIContractTradingSymbolRepository(controller),
		tierRepository:               mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
		contractIndicatorScriptProxy: mocks.NewMockIContractIndicatorScriptProxy(controller),
		tradeHistoryProxy:            mocks.NewMockIContractTradeHistoryProxy(controller),
	}
	failure(boundaries)

	confirmedAt := contractBacktestServiceStart
	fundingIntervalHours := 8
	boundaries.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{
			Symbol:                 "BTCUSDT",
			MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
			FundingIntervalHours:   &fundingIntervalHours,
			SpecificationUpdatedAt: &confirmedAt,
		}, true, nil).AnyTimes()
	boundaries.tierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return([]entities.ContractMaintenanceMarginTier{}, nil).AnyTimes()

	kCandleContracts := make([]entities.KCandleContract, 0, 2)
	for hour := range 2 {
		price := decimal.NewFromInt(100)
		kCandleContracts = append(kCandleContracts, entities.KCandleContract{
			Symbol: "BTCUSDT", OpenTime: contractBacktestServiceStart.Add(time.Duration(hour) * time.Hour),
			Open: price, High: price, Low: price, Close: price,
			MarkOpen: price, MarkHigh: price, MarkLow: price, MarkClose: price,
		})
	}
	boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(kCandleContracts, nil).AnyTimes()
	boundaries.settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractFundingRateSettlement{}, nil).AnyTimes()
	boundaries.settlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	boundaries.statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractPositionStatistic{}, nil).AnyTimes()
	boundaries.contractIndicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil).AnyTimes()

	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(contractBacktestServiceStart.Add(24 * time.Hour)).AnyTimes()

	return service.NewContractBacktestService(
		boundaries.kCandleContractRepository,
		boundaries.settlementRepository,
		boundaries.statisticRepository,
		boundaries.tradingSymbolRepository,
		boundaries.tierRepository,
		boundaries.contractIndicatorScriptProxy,
		service.NewContractOrderFlowService(boundaries.tradeHistoryProxy, clockProxy, service.NewContractOrderFlowMemory(1<<20)),
		clockProxy,
		1000,
		defaultWarmupBarCount,
	)
}

func contractBacktestServiceRequest() dto.ContractBacktestRequestDto {
	return dto.ContractBacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1h",
		StartTime:           contractBacktestServiceStart,
		EndTime:             contractBacktestServiceStart.Add(4 * time.Hour),
		Script:              "the script",
		InitialCapital:      decimal.NewFromInt(10000),
	}
}

func contractTradingStrategyBacktestServiceRequest() dto.ContractTradingStrategyBacktestRequestDto {
	return dto.ContractTradingStrategyBacktestRequestDto{
		Symbol:    "BTCUSDT",
		StartTime: contractBacktestServiceStart,
		EndTime:   contractBacktestServiceStart.Add(4 * time.Hour),
		SignalSources: []dto.ResolvedSignalSourceDto{
			{Label: "A", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle"},
		},
		BuyCondition:                  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "buy"},
		SellCondition:                 dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "sell"},
		TradingStrategyMarketDataKind: "contractKCandle",
		InitialCapital:                decimal.NewFromInt(10000),
	}
}

// contractBacktestServiceFailures fails each boundary alone; its error must reach the caller, since a replay built on a failed read is wrong, not shorter.
func contractBacktestServiceFailures() []contractBacktestServiceFailure {
	return []contractBacktestServiceFailure{
		{name: "the symbol cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
				Return(entities.ContractTradingSymbol{}, false, errStorageUnavailable)
		}},
		{name: "the ladder cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.tierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
		{name: "the contract K candles cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
		{name: "the settlements cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
		{name: "the settlement before the stretch cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.settlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.ContractFundingRateSettlement{}, false, errStorageUnavailable)
		}},
		{name: "the position statistics cannot be read", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
		{name: "the script fails", failWith: func(boundaries contractBacktestServiceMocks) {
			boundaries.contractIndicatorScriptProxy.EXPECT().
				ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
	}
}

func TestContractBacktestServiceHandsOnEveryFailedRead(t *testing.T) {
	for _, failure := range contractBacktestServiceFailures() {
		t.Run("replaying a script when "+failure.name, func(t *testing.T) {
			contractBacktestService := newContractBacktestServiceUnderTest(t, failure.failWith)

			_, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest(), nil)

			require.ErrorIs(t, err, errStorageUnavailable)
		})

		t.Run("replaying a trading strategy when "+failure.name, func(t *testing.T) {
			contractBacktestService := newContractBacktestServiceUnderTest(t, failure.failWith)

			_, err := contractBacktestService.RunContractTradingStrategyBacktest(
				context.Background(), contractTradingStrategyBacktestServiceRequest(), nil)

			require.ErrorIs(t, err, errStorageUnavailable)
		})
	}
}

func TestContractBacktestServiceRefusals(t *testing.T) {
	noFailure := func(contractBacktestServiceMocks) {}

	t.Run("a blank symbol is refused before anything is read", func(t *testing.T) {
		requestDto := contractBacktestServiceRequest()
		requestDto.Symbol = ""

		_, err := newContractBacktestServiceUnderTest(t, noFailure).RunContractBacktest(context.Background(), requestDto, nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a trading strategy on a symbol without a specification is refused", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
			boundaries.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
				Return(entities.ContractTradingSymbol{}, false, nil)
		})

		_, err := contractBacktestService.RunContractTradingStrategyBacktest(
			context.Background(), contractTradingStrategyBacktestServiceRequest(), nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a stretch too short for two bars is refused", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
			boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]entities.KCandleContract{}, nil)
		})

		_, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest(), nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a trading strategy replay refused by its own rules reads nothing", func(t *testing.T) {
		requestDto := contractTradingStrategyBacktestServiceRequest()
		requestDto.TradingMode = "shortOnly"

		_, err := newContractBacktestServiceUnderTest(t, noFailure).RunContractTradingStrategyBacktest(
			context.Background(), requestDto, nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("the funding in force before the stretch is lined up for the script but never paid", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
			boundaries.settlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.ContractFundingRateSettlement{
					Symbol: "BTCUSDT", SettlementTime: contractBacktestServiceStart.Add(-8 * time.Hour),
					FundingRate: decimal.RequireFromString("0.01"),
				}, true, nil)
		})

		resultDto, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest(), nil)

		require.NoError(t, err)
		assert.True(t, decimal.Zero.Equal(resultDto.Summary.TotalFundingFee))
	})
}

// orderFlowWidth declares an order flow price step on a replayed script.
func orderFlowWidth(defaultValue float64) []dto.StrategyScriptParameterWriteDto {
	return []dto.StrategyScriptParameterWriteDto{{Name: "寬", Kind: "orderFlowPriceStep", DefaultValue: defaultValue}}
}

// archivedTrade is one buyer-initiated trade the day's archive hands over.
func archivedTrade(at time.Time, price int64) func(context.Context, string, time.Time, func(vo.ContractAggregatedTradeVo)) (bool, error) {
	return func(_ context.Context, _ string, _ time.Time, visit func(vo.ContractAggregatedTradeVo)) (bool, error) {
		visit(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(price), Quantity: 1, TradeCount: 1, TradeTime: at, BuyerIsTaker: true})

		return true, nil
	}
}

func TestAContractReplayHandsEveryBarItsFootprint(t *testing.T) {
	replayedBars := []vo.ContractKCandleVo{}
	contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
		boundaries.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", contractBacktestServiceStart, gomock.Any()).
			DoAndReturn(archivedTrade(contractBacktestServiceStart.Add(30*time.Minute), 104))
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
				_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				replayedBars = bars

				return []map[string]vo.IndicatorValueVo{
					{vo.SignalIndicatorKey: {Signal: vo.SignalHold}}, {vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
				}, nil
			})
	})
	requestDto := contractBacktestServiceRequest()
	requestDto.Parameters = orderFlowWidth(10)

	_, err := contractBacktestService.RunContractBacktest(context.Background(), requestDto, nil)

	require.NoError(t, err)
	require.Len(t, replayedBars, 2)
	assert.Equal(t, vo.OrderFlowFootprintVo{
		PriceStep: 10, Levels: []vo.OrderFlowPriceLevelVo{{Price: 100, BuyVolume: 1, TradeCount: 1}},
		Delta: 1, PointOfControl: 100,
	}, replayedBars[0].Footprint)
	assert.Equal(t, 10.0, replayedBars[1].Footprint.PriceStep)
	assert.Empty(t, replayedBars[1].Footprint.Levels)
}

// replayingWith captures the bars each replay of a source is handed, holding every bar.
func replayingWith(captured *[][]vo.ContractKCandleVo) func(context.Context, string, domains.IndicatorResultTypeDomain,
	[]vo.ContractKCandleVo, domains.StrategyScriptParametersDomain, domaininterface.IReplayProgressSink,
) ([]map[string]vo.IndicatorValueVo, error) {
	return func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
		_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
	) ([]map[string]vo.IndicatorValueVo, error) {
		*captured = append(*captured, bars)

		return []map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}}, {vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil
	}
}

func TestEachTradingStrategySourceReadsFootprintsAtItsOwnPriceStep(t *testing.T) {
	captured := [][]vo.ContractKCandleVo{}
	contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
		boundaries.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", contractBacktestServiceStart, gomock.Any()).
			DoAndReturn(archivedTrade(contractBacktestServiceStart.Add(30*time.Minute), 104)).Times(2)
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(replayingWith(&captured)).Times(2)
	})
	requestDto := contractTradingStrategyBacktestServiceRequest()
	requestDto.SignalSources = []dto.ResolvedSignalSourceDto{
		{Label: "A", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle",
			Parameters: orderFlowWidth(10)},
		{Label: "B", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle",
			Parameters: orderFlowWidth(10), ParameterValues: []dto.StrategyScriptParameterValueDto{{Name: "寬", Value: 50}}},
	}

	_, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

	require.NoError(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, 10.0, captured[0][0].Footprint.PriceStep)
	assert.Equal(t, 100.0, captured[0][0].Footprint.Levels[0].Price)
	assert.Equal(t, 50.0, captured[1][0].Footprint.PriceStep)
	assert.Equal(t, 100.0, captured[1][0].Footprint.Levels[0].Price)
}

func TestAContractReplayWhoseOrderFlowCannotBeReadFails(t *testing.T) {
	for _, replay := range []struct {
		name string
		run  func(*service.ContractBacktestService) error
	}{
		{name: "a script", run: func(contractBacktestService *service.ContractBacktestService) error {
			requestDto := contractBacktestServiceRequest()
			requestDto.Parameters = orderFlowWidth(10)
			_, err := contractBacktestService.RunContractBacktest(context.Background(), requestDto, nil)

			return err
		}},
		{name: "a trading strategy", run: func(contractBacktestService *service.ContractBacktestService) error {
			requestDto := contractTradingStrategyBacktestServiceRequest()
			requestDto.SignalSources[0].Parameters = orderFlowWidth(10)
			_, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

			return err
		}},
	} {
		t.Run(replay.name, func(t *testing.T) {
			contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
				boundaries.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false, errStorageUnavailable)
			})

			err := replay.run(contractBacktestService)

			require.ErrorIs(t, err, domains.ErrContractOrderFlowUnavailable)
			assert.Contains(t, err.Error(), "讀不到訂單流")
		})
	}
}

func TestAContractReplayReachingPastThirtyOneDaysOfOrderFlowIsRefusedWhenSubmitted(t *testing.T) {
	longScriptRequest := contractBacktestServiceRequest()
	longScriptRequest.StartTime = contractBacktestServiceStart.AddDate(0, 0, -40)
	longScriptRequest.Parameters = orderFlowWidth(10)
	longStrategyRequest := contractTradingStrategyBacktestServiceRequest()
	longStrategyRequest.StartTime = contractBacktestServiceStart.AddDate(0, 0, -40)
	longStrategyRequest.SignalSources[0].Parameters = orderFlowWidth(10)
	shortScriptRequest := contractBacktestServiceRequest()
	shortScriptRequest.Parameters = orderFlowWidth(10)
	noOrderFlowRequest := contractBacktestServiceRequest()
	noOrderFlowRequest.StartTime = contractBacktestServiceStart.AddDate(0, 0, -40)
	contractBacktestService := newContractBacktestServiceUnderTest(t, func(contractBacktestServiceMocks) {})

	scriptError := contractBacktestService.ValidateContractBacktest(context.Background(), longScriptRequest)
	strategyError := contractBacktestService.ValidateContractTradingStrategyBacktest(context.Background(), longStrategyRequest)

	require.ErrorIs(t, scriptError, domains.ErrBacktestValidation)
	assert.Contains(t, scriptError.Error(), "訂單流一次至多涵蓋 31 天")
	require.ErrorIs(t, strategyError, domains.ErrBacktestValidation)
	assert.Contains(t, strategyError.Error(), "訂單流一次至多涵蓋 31 天")
	assert.NoError(t, contractBacktestService.ValidateContractBacktest(context.Background(), shortScriptRequest))
	assert.NoError(t, contractBacktestService.ValidateContractBacktest(context.Background(), noOrderFlowRequest),
		"沒宣告訂單流的不受 31 天限制")
}

func TestTheUpFrontOrderFlowCheckCountsTheDefaultWarmUpTheReplayWillAdd(t *testing.T) {
	// Twenty-five days asked for, plus two hundred hourly bars of default warm-up, reaches past thirty-one days.
	requestDto := contractBacktestServiceRequest()
	requestDto.StartTime = contractBacktestServiceStart.AddDate(0, 0, -25)
	requestDto.Parameters = orderFlowWidth(10)
	strategyRequest := contractTradingStrategyBacktestServiceRequest()
	strategyRequest.StartTime = contractBacktestServiceStart.AddDate(0, 0, -25)
	strategyRequest.SignalSources[0].Parameters = orderFlowWidth(10)
	warmingUp := newContractBacktestServiceWarmingUp(t, func(contractBacktestServiceMocks) {}, 200)

	scriptError := warmingUp.ValidateContractBacktest(context.Background(), requestDto)
	strategyError := warmingUp.ValidateContractTradingStrategyBacktest(context.Background(), strategyRequest)

	require.ErrorIs(t, scriptError, domains.ErrBacktestValidation)
	assert.Contains(t, scriptError.Error(), "訂單流一次至多涵蓋 31 天")
	require.ErrorIs(t, strategyError, domains.ErrBacktestValidation)
	assert.Contains(t, strategyError.Error(), "訂單流一次至多涵蓋 31 天")
}

// A paces at one hour and B reads four hours merged from the same contract K candles; B's first bar
// finishes with the fourth hour, so the replay can only go long there.
func TestAContractTradingStrategyHearsACoarserSourceOnlyOnceItsBarHasFinished(t *testing.T) {
	hourlyCandles := make([]entities.KCandleContract, 0, 8)
	for hour := range 8 {
		price := decimal.NewFromInt(int64(100 + 10*hour))
		hourlyCandles = append(hourlyCandles, entities.KCandleContract{
			Symbol: "BTCUSDT", OpenTime: contractBacktestServiceStart.Add(time.Duration(hour) * time.Hour),
			Open: price, High: price, Low: price, Close: price,
			MarkOpen: price, MarkHigh: price, MarkLow: price, MarkClose: price,
		})
	}
	answers := [][]vo.SignalVo{
		{vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalSell},
		{vo.SignalBuy, vo.SignalSell},
	}
	barCountsBySource := []int{}
	contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
		boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(hourlyCandles, nil)
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
				_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
			) ([]map[string]vo.IndicatorValueVo, error) {
				answer := answers[len(barCountsBySource)]
				barCountsBySource = append(barCountsBySource, len(bars))
				perBarIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(answer))
				for _, signal := range answer {
					perBarIndicatorValues = append(perBarIndicatorValues, map[string]vo.IndicatorValueVo{
						vo.SignalIndicatorKey: {Signal: signal},
					})
				}

				return perBarIndicatorValues, nil
			}).Times(2)
	})
	requestDto := contractTradingStrategyBacktestServiceRequest()
	requestDto.EndTime = contractBacktestServiceStart.Add(8 * time.Hour)
	requestDto.SignalSources = []dto.ResolvedSignalSourceDto{
		{Label: "A", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle"},
		{Label: "B", AggregationInterval: "4h", Script: "the script", MarketDataKind: "contractKCandle"},
	}
	requestDto.BuyCondition = dto.TradingStrategyConditionDto{Operator: "and", Conditions: []dto.TradingStrategyConditionDto{
		{SourceLabel: "A", Signal: "buy"}, {SourceLabel: "B", Signal: "buy"},
	}}

	resultDto, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

	require.NoError(t, err)
	assert.Equal(t, "1h", resultDto.Interval)
	assert.Equal(t, []int{8, 2}, barCountsBySource)
	require.NotEmpty(t, resultDto.ClosedTrades)
	assert.Equal(t, string(vo.PositionDirectionLong), resultDto.ClosedTrades[0].Direction)
	assert.Equal(t, contractBacktestServiceStart.Add(3*time.Hour), resultDto.ClosedTrades[0].EntryTime)
	assert.True(t, decimal.NewFromInt(130).Equal(resultDto.ClosedTrades[0].EntryPrice))
}

// No four-hour bar finishes within two read hours, so B holds throughout and A alone never opens.
func TestAContractTradingStrategyCoarserSourceWithNoFinishedBarHolds(t *testing.T) {
	executedBarCounts := []int{}
	contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
		boundaries.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
				_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
			) ([]map[string]vo.IndicatorValueVo, error) {
				executedBarCounts = append(executedBarCounts, len(bars))

				return []map[string]vo.IndicatorValueVo{
					{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, {vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				}, nil
			}).Times(1)
	})
	requestDto := contractTradingStrategyBacktestServiceRequest()
	requestDto.EndTime = contractBacktestServiceStart.Add(2 * time.Hour)
	requestDto.SignalSources = []dto.ResolvedSignalSourceDto{
		{Label: "A", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle"},
		{Label: "B", AggregationInterval: "4h", Script: "the script", MarketDataKind: "contractKCandle"},
	}
	requestDto.BuyCondition = dto.TradingStrategyConditionDto{Operator: "and", Conditions: []dto.TradingStrategyConditionDto{
		{SourceLabel: "A", Signal: "buy"}, {SourceLabel: "B", Signal: "buy"},
	}}

	resultDto, err := contractBacktestService.RunContractTradingStrategyBacktest(context.Background(), requestDto, nil)

	require.NoError(t, err)
	assert.Equal(t, []int{2}, executedBarCounts)
	assert.Empty(t, resultDto.ClosedTrades)
	assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
}

func TestAContractTradingStrategyRefusesACoarseLookbackBeyondOneReplay(t *testing.T) {
	requestDto := contractTradingStrategyBacktestServiceRequest()
	requestDto.SignalSources = []dto.ResolvedSignalSourceDto{
		{Label: "A", AggregationInterval: "1h", Script: "the script", MarketDataKind: "contractKCandle"},
		{Label: "B", AggregationInterval: "1d", Script: "the script", MarketDataKind: "contractKCandle",
			Parameters: []dto.StrategyScriptParameterWriteDto{{Name: "回看", Kind: "lookbackCount", DefaultValue: 50}}},
	}

	_, err := newContractBacktestServiceUnderTest(t, func(contractBacktestServiceMocks) {}).RunContractTradingStrategyBacktest(
		context.Background(), requestDto, nil)

	require.ErrorIs(t, err, domains.ErrBacktestValidation)
	assert.ErrorContains(t, err, "要 1200 根")
	assert.ErrorContains(t, err, "最多 1000 根")
}
