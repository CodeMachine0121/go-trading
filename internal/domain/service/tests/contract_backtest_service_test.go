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
}

type contractBacktestServiceFailure struct {
	name     string
	failWith func(contractBacktestServiceMocks)
}

func newContractBacktestServiceUnderTest(
	t *testing.T, failure func(contractBacktestServiceMocks),
) *service.ContractBacktestService {
	return newContractBacktestServiceWithin(t, time.Minute, failure)
}

func newContractBacktestServiceWithin(
	t *testing.T, replayTimeAllowance time.Duration, failure func(contractBacktestServiceMocks),
) *service.ContractBacktestService {
	controller := gomock.NewController(t)
	boundaries := contractBacktestServiceMocks{
		kCandleContractRepository:    mocks.NewMockIKCandleContractRepository(controller),
		settlementRepository:         mocks.NewMockIContractFundingRateSettlementRepository(controller),
		statisticRepository:          mocks.NewMockIContractPositionStatisticRepository(controller),
		tradingSymbolRepository:      mocks.NewMockIContractTradingSymbolRepository(controller),
		tierRepository:               mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
		contractIndicatorScriptProxy: mocks.NewMockIContractIndicatorScriptProxy(controller),
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
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
		clockProxy,
		1000,
		replayTimeAllowance,
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
				ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errStorageUnavailable)
		}},
	}
}

func TestContractBacktestServiceHandsOnEveryFailedRead(t *testing.T) {
	for _, failure := range contractBacktestServiceFailures() {
		t.Run("replaying a script when "+failure.name, func(t *testing.T) {
			contractBacktestService := newContractBacktestServiceUnderTest(t, failure.failWith)

			_, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest())

			require.ErrorIs(t, err, errStorageUnavailable)
		})

		t.Run("replaying a trading strategy when "+failure.name, func(t *testing.T) {
			contractBacktestService := newContractBacktestServiceUnderTest(t, failure.failWith)

			_, err := contractBacktestService.RunContractTradingStrategyBacktest(
				context.Background(), contractTradingStrategyBacktestServiceRequest())

			require.ErrorIs(t, err, errStorageUnavailable)
		})
	}
}

func TestContractBacktestServiceRefusals(t *testing.T) {
	noFailure := func(contractBacktestServiceMocks) {}

	t.Run("a blank symbol is refused before anything is read", func(t *testing.T) {
		requestDto := contractBacktestServiceRequest()
		requestDto.Symbol = ""

		_, err := newContractBacktestServiceUnderTest(t, noFailure).RunContractBacktest(context.Background(), requestDto)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a trading strategy on a symbol without a specification is refused", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
			boundaries.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
				Return(entities.ContractTradingSymbol{}, false, nil)
		})

		_, err := contractBacktestService.RunContractTradingStrategyBacktest(
			context.Background(), contractTradingStrategyBacktestServiceRequest())

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a stretch too short for two bars is refused", func(t *testing.T) {
		contractBacktestService := newContractBacktestServiceUnderTest(t, func(boundaries contractBacktestServiceMocks) {
			boundaries.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]entities.KCandleContract{}, nil)
		})

		_, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest())

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a trading strategy replay refused by its own rules reads nothing", func(t *testing.T) {
		requestDto := contractTradingStrategyBacktestServiceRequest()
		requestDto.TradingMode = "shortOnly"

		_, err := newContractBacktestServiceUnderTest(t, noFailure).RunContractTradingStrategyBacktest(
			context.Background(), requestDto)

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

		resultDto, err := contractBacktestService.RunContractBacktest(context.Background(), contractBacktestServiceRequest())

		require.NoError(t, err)
		assert.True(t, decimal.Zero.Equal(resultDto.Summary.TotalFundingFee))
	})
}
