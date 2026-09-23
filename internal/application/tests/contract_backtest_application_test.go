package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
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

const (
	contractReplayScriptID          = uint(21)
	contractReplayKCandleScriptID   = uint(22)
	contractReplayTradingStrategyID = uint(31)
)

type contractBacktestUnderTest struct {
	backtestApplication                *application.BacktestApplication
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication
	kCandleContractRepository          *mocks.MockIKCandleContractRepository
	contractTradingSymbolRepository    *mocks.MockIContractTradingSymbolRepository
	contractIndicatorScriptProxy       *mocks.MockIContractIndicatorScriptProxy
	tradingStrategyRepository          *mocks.MockITradingStrategyRepository
}

// newContractBacktestUnderTest wires the real domain services and models, mocking only
// storage, the script runner and the clock. The symbol's funding, positioning and
// ladder are empty unless a test says otherwise.
func newContractBacktestUnderTest(t *testing.T) contractBacktestUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(controller)
	contractFundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractFundingRateSettlement{}, nil).AnyTimes()
	contractFundingRateSettlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	contractPositionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	contractPositionStatisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractPositionStatistic{}, nil).AnyTimes()
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(controller)
	contractMaintenanceMarginTierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(controller)
	contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return([]entities.ContractMaintenanceMarginTier{}, nil).AnyTimes()
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(controller)

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			marketDataKind := string(vo.MarketDataKindContractKCandle)
			if id == contractReplayKCandleScriptID {
				marketDataKind = string(vo.MarketDataKindKCandle)
			}

			return entities.StrategyScript{
				ID: id, OwnerID: backtestViewerID, Script: "the script", MarketDataKind: marketDataKind,
			}, nil
		}).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)

	strategyScriptService := service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)
	backtestService := service.NewBacktestService(kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, time.Minute)
	contractBacktestService := service.NewContractBacktestService(
		kCandleContractRepository,
		contractFundingRateSettlementRepository,
		contractPositionStatisticRepository,
		contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository,
		contractIndicatorScriptProxy,
		clockProxy,
		queryMaxResults,
		time.Minute,
	)

	return contractBacktestUnderTest{
		backtestApplication: application.NewBacktestApplication(
			strategyScriptService, backtestService, contractBacktestService),
		tradingStrategyBacktestApplication: application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			strategyScriptService, backtestService, contractBacktestService),
		kCandleContractRepository:       kCandleContractRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		contractIndicatorScriptProxy:    contractIndicatorScriptProxy,
		tradingStrategyRepository:       tradingStrategyRepository,
	}
}

func (fixture contractBacktestUnderTest) expectTheSymbolIsSpecified() {
	confirmedAt := backtestStart
	fundingIntervalHours := 8
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{
			Symbol:                 "BTCUSDT",
			TickSize:               decimal.NewNullDecimal(decimal.RequireFromString("0.01")),
			QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			MinimumQuantity:        decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			MinimumNotional:        decimal.NewNullDecimal(decimal.RequireFromString("5")),
			MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
			LiquidationFeeRate:     decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
			FundingIntervalHours:   &fundingIntervalHours,
			SpecificationUpdatedAt: &confirmedAt,
		}, true, nil)
}

// expectHourlyContractBars stores one contract K candle per hour at these closes.
func (fixture contractBacktestUnderTest) expectHourlyContractBars(closes ...string) {
	kCandleContracts := make([]entities.KCandleContract, 0, len(closes))
	for hour, closePrice := range closes {
		price := decimal.RequireFromString(closePrice)
		kCandleContracts = append(kCandleContracts, entities.KCandleContract{
			Symbol:   "BTCUSDT",
			OpenTime: backtestStart.Add(time.Duration(hour) * time.Hour),
			Open:     price, High: price, Low: price, Close: price,
			MarkOpen: price, MarkHigh: price, MarkLow: price, MarkClose: price,
		})
	}
	fixture.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(kCandleContracts, nil)
}

func contractBacktestRequestDto() dto.ContractBacktestRequestDto {
	return dto.ContractBacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1h",
		StartTime:           backtestStart,
		EndTime:             backtestStart.Add(4 * time.Hour),
		InitialCapital:      decimal.NewFromInt(10000),
		Leverage:            decimal.NewFromInt(5),
	}
}

func TestRunContractBacktest(t *testing.T) {
	t.Run("replays a named contract strategy script on a contract account", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.expectTheSymbolIsSpecified()
		fixture.expectHourlyContractBars("100", "110")
		fixture.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Len(2), gomock.Any()).
			Return(signalsSaying(vo.SignalBuy, vo.SignalSell), nil)

		resultDto, err := fixture.backtestApplication.RunContractBacktest(context.Background(), backtestViewerID,
			namingStrategyScript(t, contractReplayScriptID), contractBacktestRequestDto())

		require.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, "1h", resultDto.Interval)
		assert.Equal(t, "longShort", resultDto.TradingMode)
		require.Len(t, resultDto.ClosedTrades, 1)
		// 500 units bought at 100, sold at 110.
		assert.True(t, decimal.NewFromInt(5000).Equal(resultDto.ClosedTrades[0].Profit))
		assert.Equal(t, 2, resultDto.Summary.PositionOpenCount)
	})

	t.Run("a K candle strategy script is refused for what it is", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)

		_, err := fixture.backtestApplication.RunContractBacktest(context.Background(), backtestViewerID,
			namingStrategyScript(t, contractReplayKCandleScriptID), contractBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrStrategyScriptMarketDataKindMismatch)
		assert.Contains(t, err.Error(), "吃的是 K 線")
	})

	t.Run("a symbol without a trading specification is refused", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
			Return(entities.ContractTradingSymbol{}, false, nil)

		_, err := fixture.backtestApplication.RunContractBacktest(context.Background(), backtestViewerID,
			namingStrategyScript(t, contractReplayScriptID), contractBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "還沒有交易規格")
	})

	t.Run("a spot replay of a contract strategy script is refused for what it is", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)

		_, err := fixture.backtestApplication.RunBacktest(context.Background(), backtestViewerID,
			namingStrategyScript(t, contractReplayScriptID), backtestRequestDto())

		require.ErrorIs(t, err, domains.ErrStrategyScriptMarketDataKindMismatch)
		assert.Contains(t, err.Error(), "吃的是合約行情")
	})
}

// aContractTradingStrategy is one contract source A buying and selling on its own word.
func aContractTradingStrategy(marketDataKind string, tradingMode string) entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: contractReplayTradingStrategyID, OwnerID: backtestViewerID, Name: "合約黃金交叉",
		MarketDataKind: marketDataKind,
		TradingMode:    tradingMode,
		SignalSources: []entities.TradingStrategySignalSource{{
			ID: 40, TradingStrategyID: contractReplayTradingStrategyID,
			Label: "A", StrategyScriptID: contractReplayScriptID, AggregationInterval: "1h",
		}},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 41, TradingStrategyID: contractReplayTradingStrategyID, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 42, TradingStrategyID: contractReplayTradingStrategyID, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

func contractTradingStrategyBacktestRequestDto() dto.ContractTradingStrategyBacktestRequestDto {
	return dto.ContractTradingStrategyBacktestRequestDto{
		Symbol:         "BTCUSDT",
		StartTime:      backtestStart,
		EndTime:        backtestStart.Add(4 * time.Hour),
		InitialCapital: decimal.NewFromInt(10000),
		Leverage:       decimal.NewFromInt(5),
	}
}

func TestRunContractTradingStrategyBacktest(t *testing.T) {
	t.Run("replays a contract trading strategy by its own trading mode", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "shortOnly"), nil)
		fixture.expectTheSymbolIsSpecified()
		fixture.expectHourlyContractBars("100", "90")
		fixture.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Len(2), gomock.Any()).
			Return(signalsSaying(vo.SignalSell, vo.SignalBuy), nil)

		resultDto, err := fixture.tradingStrategyBacktestApplication.RunContractTradingStrategyBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID,
			contractTradingStrategyBacktestRequestDto())

		require.NoError(t, err)
		assert.Equal(t, "shortOnly", resultDto.TradingMode)
		// The coarseness is the one the sources share, and every bar is on the curve.
		assert.Equal(t, "1h", resultDto.Interval)
		assert.Len(t, resultDto.EquityCurve, 2)
		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "short", resultDto.ClosedTrades[0].Direction)
		// 500 units shorted at 100, bought back at 90.
		assert.True(t, decimal.NewFromInt(5000).Equal(resultDto.ClosedTrades[0].Profit))
		// Short only: the buy closed it and opened nothing.
		assert.Equal(t, 1, resultDto.Summary.PositionOpenCount)
	})

	t.Run("a K candle trading strategy cannot be replayed on a contract account", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("", ""), nil)
		fixture.expectTheSymbolIsSpecified()

		_, err := fixture.tradingStrategyBacktestApplication.RunContractTradingStrategyBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID,
			contractTradingStrategyBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "吃的是 K 線")
	})

	t.Run("a contract trading strategy cannot be replayed on spot", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "longShort"), nil)

		_, err := fixture.tradingStrategyBacktestApplication.RunTradingStrategyBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID,
			tradingStrategyBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "吃的是合約行情")
	})
}

func TestRunContractBacktestEdges(t *testing.T) {
	t.Run("a script written in the request runs as written", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.expectTheSymbolIsSpecified()
		fixture.expectHourlyContractBars("100", "110")
		fixture.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "written here", gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(vo.SignalHold, vo.SignalHold), nil)
		runSubject, err := domains.NewRunSubjectDomain(0, "written here", "", nil)
		require.NoError(t, err)

		resultDto, err := fixture.backtestApplication.RunContractBacktest(
			context.Background(), backtestViewerID, runSubject, contractBacktestRequestDto())

		require.NoError(t, err)
		assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
	})

	t.Run("a named script whose kind cannot be read is refused", func(t *testing.T) {
		controller := gomock.NewController(t)
		strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
			Return(entities.StrategyScript{ID: 5, OwnerID: backtestViewerID, MarketDataKind: "option"}, nil)
		backtestApplication := application.NewBacktestApplication(
			service.NewStrategyScriptService(strategyScriptRepository,
				mocks.NewMockIPublishedStrategyScriptRepository(controller)), nil, nil)

		_, err := backtestApplication.RunContractBacktest(context.Background(), backtestViewerID,
			namingStrategyScript(t, 5), contractBacktestRequestDto())

		assert.Error(t, err)
	})

	t.Run("a contract trading strategy that is not this person's is not found", func(t *testing.T) {
		fixture := newContractBacktestUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(entities.TradingStrategy{}, domains.ErrTradingStrategyNotFound)

		_, err := fixture.tradingStrategyBacktestApplication.RunContractTradingStrategyBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID,
			contractTradingStrategyBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrTradingStrategyNotFound)
	})

	t.Run("a contract trading strategy naming a script nobody may read is not found", func(t *testing.T) {
		controller := gomock.NewController(t)
		tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "longShort"), nil)
		strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
			Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)
		tradingStrategyBacktestApplication := application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(strategyScriptRepository,
				mocks.NewMockIPublishedStrategyScriptRepository(controller)),
			nil, nil)

		_, err := tradingStrategyBacktestApplication.RunContractTradingStrategyBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID,
			contractTradingStrategyBacktestRequestDto())

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}
