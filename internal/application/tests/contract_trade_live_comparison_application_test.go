package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type liveComparisonUnderTest struct {
	application                     *application.ContractTradeLiveComparisonApplication
	contractTradeRecordRepository   *mocks.MockIContractTradeRecordRepository
	tradingStrategyRepository       *mocks.MockITradingStrategyRepository
	strategyScriptRepository        *mocks.MockIStrategyScriptRepository
	contractTradingSymbolRepository *mocks.MockIContractTradingSymbolRepository
	kCandleContractRepository       *mocks.MockIKCandleContractRepository
	contractIndicatorScriptProxy    *mocks.MockIContractIndicatorScriptProxy
}

func newLiveComparisonUnderTest(t *testing.T) liveComparisonUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestStart.Add(30 * 24 * time.Hour)).AnyTimes()
	fixture := liveComparisonUnderTest{
		contractTradeRecordRepository:   mocks.NewMockIContractTradeRecordRepository(controller),
		tradingStrategyRepository:       mocks.NewMockITradingStrategyRepository(controller),
		strategyScriptRepository:        mocks.NewMockIStrategyScriptRepository(controller),
		contractTradingSymbolRepository: mocks.NewMockIContractTradingSymbolRepository(controller),
		kCandleContractRepository:       mocks.NewMockIKCandleContractRepository(controller),
		contractIndicatorScriptProxy:    mocks.NewMockIContractIndicatorScriptProxy(controller),
	}
	fundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	fundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractFundingRateSettlement{}, nil).AnyTimes()
	fundingRateSettlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	positionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	positionStatisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractPositionStatistic{}, nil).AnyTimes()
	maintenanceMarginTierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(controller)
	maintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return([]entities.ContractMaintenanceMarginTier{}, nil).AnyTimes()
	tradeJournalSettingRepository := mocks.NewMockITradeJournalSettingRepository(controller)
	tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), backtestViewerID).
		Return(entities.TradeJournalSetting{TakerFeeRate: percentage("0.05")}, true, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	fixture.application = application.NewContractTradeLiveComparisonApplication(
		service.NewContractTradeJournalService(
			fixture.contractTradeRecordRepository, mocks.NewMockITradeTagRepository(controller),
			tradeJournalSettingRepository, fixture.tradingStrategyRepository, fixture.contractTradingSymbolRepository,
			maintenanceMarginTierRepository, fundingRateSettlementRepository, fixture.kCandleContractRepository,
			mocks.NewMockIStrategyBotRepository(controller), mocks.NewMockIStrategyBotRunRecordRepository(controller),
			clockProxy),
		service.NewTradingStrategyService(fixture.tradingStrategyRepository),
		service.NewStrategyScriptService(fixture.strategyScriptRepository, publishedStrategyScriptRepository),
		service.NewContractBacktestService(
			fixture.kCandleContractRepository, fundingRateSettlementRepository, positionStatisticRepository,
			fixture.contractTradingSymbolRepository, maintenanceMarginTierRepository,
			fixture.contractIndicatorScriptProxy, clockProxy, queryMaxResults, time.Minute),
	)

	return fixture
}

func (fixture liveComparisonUnderTest) theScriptIsOwned() {
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), contractReplayScriptID).Return(entities.StrategyScript{
		ID: contractReplayScriptID, OwnerID: backtestViewerID, Script: "the script",
		MarketDataKind: string(vo.MarketDataKindContractKCandle),
	}, nil).AnyTimes()
}

func aLinkedClosedTrade(id uint, symbol string, direction string, entry string, exit string, leverage int64, closedDay int) entities.ContractTradeRecord {
	tradingStrategyID := contractReplayTradingStrategyID
	openedAt := backtestStart.AddDate(0, 0, closedDay-1)
	closedAt := backtestStart.AddDate(0, 0, closedDay)

	return entities.ContractTradeRecord{
		ID: id, OwnerID: backtestViewerID, Symbol: symbol, Direction: direction, Leverage: decimal.NewFromInt(leverage),
		Status: string(vo.ContractTradeStatusClosed), OpenedAt: openedAt, ClosedAt: &closedAt,
		TradingStrategyID: &tradingStrategyID,
		Fills: []entities.ContractTradeFill{
			journalFill(id*10+1, vo.ContractTradeFillKindEntry, openedAt, entry, "1"),
			journalFill(id*10+2, vo.ContractTradeFillKindExit, closedAt, exit, "1"),
		},
	}
}

func TestContractTradeLiveComparisonApplication(t *testing.T) {
	t.Run("each symbol is replayed over the stretch its trades covered", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		fixture.theScriptIsOwned()
		fixture.contractTradeRecordRepository.EXPECT().
			FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, contractReplayTradingStrategyID).
			Return([]entities.ContractTradeRecord{
				aLinkedClosedTrade(1, "BTCUSDT", "short", "100", "90", 10, 2),
				aLinkedClosedTrade(2, "BTCUSDT", "long", "100", "95", 5, 3),
				aLinkedClosedTrade(3, "ETHUSDT", "long", "3000", "3100", 5, 4),
			}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "shortOnly"), nil)
		specifiedAt := backtestStart
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(entities.ContractTradingSymbol{
			Symbol: "BTCUSDT", TickSize: percentage("0.01"), QuantityStep: percentage("0.001"),
			MinimumQuantity: percentage("0.001"), MinimumNotional: percentage("5"),
			MaintenanceMarginRate: percentage("0.005"), SpecificationUpdatedAt: &specifiedAt,
		}, true, nil).AnyTimes()
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").
			Return(entities.ContractTradingSymbol{}, false, nil).AnyTimes()
		fixture.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(
			[]entities.KCandleContract{
				{Symbol: "BTCUSDT", OpenTime: backtestStart, Open: decimal.NewFromInt(100), High: decimal.NewFromInt(100),
					Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100), MarkOpen: decimal.NewFromInt(100),
					MarkHigh: decimal.NewFromInt(100), MarkLow: decimal.NewFromInt(100), MarkClose: decimal.NewFromInt(100)},
				{Symbol: "BTCUSDT", OpenTime: backtestStart.Add(time.Hour), Open: decimal.NewFromInt(90), High: decimal.NewFromInt(90),
					Low: decimal.NewFromInt(90), Close: decimal.NewFromInt(90), MarkOpen: decimal.NewFromInt(90),
					MarkHigh: decimal.NewFromInt(90), MarkLow: decimal.NewFromInt(90), MarkClose: decimal.NewFromInt(90)},
			}, nil)
		fixture.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Len(2), gomock.Any()).
			Return(signalsSaying(vo.SignalSell, vo.SignalBuy), nil)

		comparison, err := fixture.application.CompareWithBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.NoError(t, err)
		assert.Equal(t, "合約黃金交叉", comparison.TradingStrategyName)
		assert.False(t, comparison.NoClosedTrades)
		require.Len(t, comparison.Rows, 2)
		bitcoin := comparison.Rows[0]
		assert.Equal(t, "BTCUSDT", bitcoin.Symbol)
		assert.True(t, bitcoin.StartTime.Equal(backtestStart.AddDate(0, 0, 1)))
		assert.True(t, bitcoin.EndTime.Equal(backtestStart.AddDate(0, 0, 3)))
		assert.Equal(t, "5", bitcoin.Leverage.String())
		assert.Equal(t, 2, bitcoin.Live.ClosedTradeCount)
		assert.InDelta(t, 0.5, *bitcoin.Live.WinRate, 0.0001)
		assert.InDelta(t, 1.0, *bitcoin.Live.ShortWinRate, 0.0001)
		assert.InDelta(t, 0.0, *bitcoin.Live.LongWinRate, 0.0001)
		require.NotNil(t, bitcoin.Backtest)
		assert.Equal(t, 1, bitcoin.Backtest.ClosedTradeCount)
		assert.InDelta(t, 1.0, *bitcoin.Backtest.WinRate, 0.0001)
		ethereum := comparison.Rows[1]
		assert.Equal(t, "ETHUSDT", ethereum.Symbol)
		assert.Equal(t, 1, ethereum.Live.ClosedTradeCount)
		assert.Nil(t, ethereum.Live.ShortWinRate)
		assert.Nil(t, ethereum.Backtest)
		assert.Contains(t, ethereum.BacktestUnavailableReason, "還沒有交易規格")
	})

	t.Run("a deleted strategy still shows its live figures, without replaying", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
			Return(entities.ContractTradingSymbol{}, true, nil).AnyTimes()
		fixture.contractTradeRecordRepository.EXPECT().
			FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, contractReplayTradingStrategyID).
			Return([]entities.ContractTradeRecord{aLinkedClosedTrade(1, "BTCUSDT", "long", "100", "110", 5, 2)}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(contractReplayTradingStrategyID))

		comparison, err := fixture.application.CompareWithBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.NoError(t, err)
		assert.True(t, comparison.TradingStrategyDeleted)
		require.Len(t, comparison.Rows, 1)
		assert.InDelta(t, 1.0, *comparison.Rows[0].Live.WinRate, 0.0001)
		assert.Equal(t, "交易策略已刪除，無法重演", comparison.Rows[0].BacktestUnavailableReason)
	})

	t.Run("a strategy with no closed trades is not replayed", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().
			FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, contractReplayTradingStrategyID).
			Return(nil, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "longShort"), nil)
		fixture.theScriptIsOwned()

		comparison, err := fixture.application.CompareWithBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.NoError(t, err)
		assert.True(t, comparison.NoClosedTrades)
		assert.Empty(t, comparison.Rows)
	})

	t.Run("a strategy that is not the person's and has no trades is not found", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().
			FindClosedByOwnerAndTradingStrategy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(contractReplayTradingStrategyID))

		_, err := fixture.application.CompareWithBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.ErrorIs(t, err, domains.ErrTradingStrategyNotFound)
	})

	t.Run("sources that cannot be resolved leave every row without a replay", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
			Return(entities.ContractTradingSymbol{}, true, nil).AnyTimes()
		fixture.contractTradeRecordRepository.EXPECT().
			FindClosedByOwnerAndTradingStrategy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.ContractTradeRecord{aLinkedClosedTrade(1, "BTCUSDT", "long", "100", "110", 5, 2)}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(aContractTradingStrategy("contractKCandle", "longShort"), nil)
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), contractReplayScriptID).
			Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(contractReplayScriptID))

		comparison, err := fixture.application.CompareWithBacktest(
			context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.NoError(t, err)
		require.Len(t, comparison.Rows, 1)
		assert.Nil(t, comparison.Rows[0].Backtest)
		assert.NotEmpty(t, comparison.Rows[0].BacktestUnavailableReason)
	})

	t.Run("fee rates that cannot be read stop the comparison", func(t *testing.T) {
		controller := gomock.NewController(t)
		recordRepository := mocks.NewMockIContractTradeRecordRepository(controller)
		recordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		settingRepository := mocks.NewMockITradeJournalSettingRepository(controller)
		settingRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).Return(entities.TradeJournalSetting{}, false, errStorageDown)
		journalService := service.NewContractTradeJournalService(
			recordRepository, mocks.NewMockITradeTagRepository(controller), settingRepository,
			mocks.NewMockITradingStrategyRepository(controller), mocks.NewMockIContractTradingSymbolRepository(controller),
			mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
			mocks.NewMockIContractFundingRateSettlementRepository(controller), mocks.NewMockIKCandleContractRepository(controller),
			mocks.NewMockIStrategyBotRepository(controller), mocks.NewMockIStrategyBotRunRecordRepository(controller),
			mocks.NewMockIClockProxy(controller))

		_, err := journalService.ListComparableGroups(context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.ErrorIs(t, err, errStorageDown)
	})

	t.Run("failures reading trades or the strategy come back", func(t *testing.T) {
		fixture := newLiveComparisonUnderTest(t)
		gomock.InOrder(
			fixture.contractTradeRecordRepository.EXPECT().
				FindClosedByOwnerAndTradingStrategy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errStorageDown),
			fixture.contractTradeRecordRepository.EXPECT().
				FindClosedByOwnerAndTradingStrategy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
		)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractReplayTradingStrategyID).
			Return(entities.TradingStrategy{}, errStorageDown)

		_, tradesError := fixture.application.CompareWithBacktest(context.Background(), backtestViewerID, contractReplayTradingStrategyID)
		_, strategyError := fixture.application.CompareWithBacktest(context.Background(), backtestViewerID, contractReplayTradingStrategyID)

		require.ErrorIs(t, tradesError, errStorageDown)
		require.ErrorIs(t, strategyError, errStorageDown)
	})
}
