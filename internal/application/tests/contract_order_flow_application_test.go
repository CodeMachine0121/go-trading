package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// orderFlowToday is just past 09:15 on 2026-10-14, after the 09:14 minute's trades have settled on the live venue; the day before has ended, and its archive may or may not be out.
var orderFlowToday = time.Date(2026, 10, 14, 9, 15, 5, 0, time.UTC)

func orderFlowDay(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
}

type orderFlowUnderTest struct {
	indicatorCalculationApplication *application.IndicatorCalculationApplication
	kCandleContractRepository       *mocks.MockIKCandleContractRepository
	contractIndicatorScriptProxy    *mocks.MockIContractIndicatorScriptProxy
	tradeHistoryProxy               *mocks.MockIContractTradeHistoryProxy
	now                             *time.Time
	lock                            *sync.Mutex
	// footprintsSeen holds the footprints of every bar handed to the script, one list per run.
	footprintsSeen *[][]vo.OrderFlowFootprintVo
}

// newOrderFlowUnderTest wires the real contract calculation and order flow services; only storage, the clock, the script runner and the trade history are mocked.
func newOrderFlowUnderTest(t *testing.T, memoryBudgetBytes int64) orderFlowUnderTest {
	controller := gomock.NewController(t)
	now := orderFlowToday
	fixture := orderFlowUnderTest{
		kCandleContractRepository:    mocks.NewMockIKCandleContractRepository(controller),
		contractIndicatorScriptProxy: mocks.NewMockIContractIndicatorScriptProxy(controller),
		tradeHistoryProxy:            mocks.NewMockIContractTradeHistoryProxy(controller),
		now:                          &now,
		lock:                         &sync.Mutex{},
		footprintsSeen:               &[][]vo.OrderFlowFootprintVo{},
	}
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		fixture.lock.Lock()
		defer fixture.lock.Unlock()

		return *fixture.now
	}).AnyTimes()

	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	settlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
			_ domains.StrategyScriptParametersDomain) (map[string]vo.IndicatorValueVo, error) {
			footprints := make([]vo.OrderFlowFootprintVo, 0, len(bars))
			for _, bar := range bars {
				footprints = append(footprints, bar.Footprint)
			}
			fixture.lock.Lock()
			defer fixture.lock.Unlock()
			*fixture.footprintsSeen = append(*fixture.footprintsSeen, footprints)

			return map[string]vo.IndicatorValueVo{"seen": {Numbers: []float64{1}}}, nil
		}).AnyTimes()

	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})
	fixture.indicatorCalculationApplication = application.NewIndicatorCalculationApplication(
		service.NewStrategyScriptService(mocks.NewMockIStrategyScriptRepository(controller),
			mocks.NewMockIPublishedStrategyScriptRepository(controller)),
		nil,
		service.NewContractIndicatorCalculationService(
			fixture.kCandleContractRepository, settlementRepository, statisticRepository,
			fixture.contractIndicatorScriptProxy,
			service.NewContractOrderFlowService(fixture.tradeHistoryProxy, clockProxy,
				service.NewContractOrderFlowMemory(memoryBudgetBytes)),
			clockProxy, marketCatalog, 100_000))

	return fixture
}

func (fixture orderFlowUnderTest) setNow(moment time.Time) {
	fixture.lock.Lock()
	defer fixture.lock.Unlock()
	*fixture.now = moment
}

// storesContractCandles answers every read with the stored contract K candles at those moments before the read's cutoff, newest first.
func (fixture orderFlowUnderTest) storesContractCandles(openTimes ...time.Time) {
	fixture.kCandleContractRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cutoff time.Time, _ int) ([]entities.KCandleContract, error) {
			newestFirst := make([]entities.KCandleContract, 0, len(openTimes))
			for index := len(openTimes) - 1; index >= 0; index-- {
				if openTimes[index].Before(cutoff) {
					newestFirst = append(newestFirst, storedContractCandlesAt(openTimes[index])...)
				}
			}

			return newestFirst, nil
		}).AnyTimes()
}

func (fixture orderFlowUnderTest) lastFootprints() []vo.OrderFlowFootprintVo {
	fixture.lock.Lock()
	defer fixture.lock.Unlock()

	return (*fixture.footprintsSeen)[len(*fixture.footprintsSeen)-1]
}

// carryingOrderFlow is an unsaved contract algorithm declaring an order flow price step.
func carryingOrderFlow(t *testing.T, priceStep float64) domains.RunSubjectDomain {
	t.Helper()

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(0, "order flow script", "", []dto.StrategyScriptParameterWriteDto{
		{Name: "寬", Kind: "orderFlowPriceStep", DefaultValue: priceStep},
	})
	require.NoError(t, subjectError)

	return runSubjectDomain
}

func orderFlowRequest(interval string, start time.Time, end time.Time) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{Symbol: "BTCUSDT", AggregationInterval: interval, StartTime: start, EndTime: end}
}

type tradeVisit = func(vo.ContractAggregatedTradeVo)

func buyerTrade(at time.Time, price int64) vo.ContractAggregatedTradeVo {
	return vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(price), Quantity: 1, TradeCount: 1, TradeTime: at, BuyerIsTaker: true}
}

func archiveHolding(trades ...vo.ContractAggregatedTradeVo) func(context.Context, string, time.Time, tradeVisit) (bool, error) {
	return func(_ context.Context, _ string, _ time.Time, visit tradeVisit) (bool, error) {
		for _, trade := range trades {
			visit(trade)
		}

		return true, nil
	}
}

func liveHolding(trades ...vo.ContractAggregatedTradeVo) func(context.Context, string, time.Time, time.Time, tradeVisit) error {
	return func(_ context.Context, _ string, _ time.Time, _ time.Time, visit tradeVisit) error {
		for _, trade := range trades {
			visit(trade)
		}

		return nil
	}
}

func TestTodaysBarsCarryFootprintsAtThisRunsPriceStep(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(at14(9, 13), at14(9, 14))
	fixture.tradeHistoryProxy.EXPECT().
		VisitTradesBetween(gomock.Any(), "BTCUSDT", orderFlowDay(10, 14), at14(9, 15), gomock.Any()).
		DoAndReturn(liveHolding(buyerTrade(at14(9, 13).Add(10*time.Second), 84160)))
	requestDto := orderFlowRequest("1m", at14(9, 13), orderFlowToday)
	requestDto.ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "寬", Value: 25}}

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	require.NoError(t, err)
	assert.Equal(t, []vo.OrderFlowFootprintVo{
		{PriceStep: 25, Levels: []vo.OrderFlowPriceLevelVo{{Price: 84150, BuyVolume: 1, TradeCount: 1}}, Delta: 1, PointOfControl: 84150},
		{PriceStep: 25, Levels: []vo.OrderFlowPriceLevelVo{}},
	}, fixture.lastFootprints())
}

func TestAScriptWithoutAnOrderFlowPriceStepAsksForNoTrades(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(at14(9, 14))

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carrying(t, "no order flow"), orderFlowRequest("1m", at14(9, 14), orderFlowToday))

	require.NoError(t, err)
	assert.Equal(t, []vo.OrderFlowFootprintVo{{}}, fixture.lastFootprints())
}

func TestEndedDaysComeFromTheirArchive(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	days := make([]time.Time, 0, 10)
	for day := 1; day <= 10; day++ {
		days = append(days, orderFlowDay(10, day))
		fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, day), gomock.Any()).
			DoAndReturn(archiveHolding(buyerTrade(orderFlowDay(10, day).Add(time.Hour), int64(100*day))))
	}
	fixture.storesContractCandles(days...)

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("1d", orderFlowDay(10, 1), orderFlowDay(10, 11)))

	require.NoError(t, err)
	footprints := fixture.lastFootprints()
	require.Len(t, footprints, 10)
	assert.Equal(t, 100.0, footprints[0].PointOfControl)
	assert.Equal(t, 1000.0, footprints[9].PointOfControl)
}

func TestYesterdayWithoutItsArchiveYetIsReadLive(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	lateYesterday := orderFlowDay(10, 13).Add(23 * time.Hour)
	fixture.storesContractCandles(lateYesterday)
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 13), gomock.Any()).
		Return(false, nil)
	fixture.tradeHistoryProxy.EXPECT().
		VisitTradesBetween(gomock.Any(), "BTCUSDT", orderFlowDay(10, 13), orderFlowDay(10, 14), gomock.Any()).
		DoAndReturn(liveHolding(buyerTrade(lateYesterday.Add(time.Minute), 84160)))

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("1h", lateYesterday, orderFlowDay(10, 14)))

	require.NoError(t, err)
	assert.Equal(t, 84160.0, fixture.lastFootprints()[0].PointOfControl)
}

func TestAnOlderDayWithoutAnArchiveCannotBeRead(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 11).Add(22 * time.Hour))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 11), gomock.Any()).
		Return(false, nil)

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10),
		orderFlowRequest("1h", orderFlowDay(10, 11).Add(22*time.Hour), orderFlowDay(10, 11).Add(23*time.Hour)))

	require.ErrorIs(t, err, domains.ErrContractOrderFlowUnavailable)
	assert.Contains(t, err.Error(), "讀不到訂單流：BTCUSDT 的 2026-10-11 歷史檔不存在")
}

func TestATradeHistoryThatWillNotAnswerFailsTheWholeCalculation(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, errors.New("archive answered 503"))

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour)))

	require.ErrorIs(t, err, domains.ErrContractOrderFlowUnavailable)
	assert.Contains(t, err.Error(), "讀不到訂單流：archive answered 503")
	assert.Empty(t, *fixture.footprintsSeen, "算式一次都沒有執行")
}

func TestOrderFlowPastThirtyOneDaysIsRefusedBeforeAskingForAnyTrade(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	days := make([]time.Time, 0, 32)
	for offset := range 32 {
		days = append(days, orderFlowDay(9, 1).AddDate(0, 0, offset))
	}
	fixture.storesContractCandles(days...)

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("1d", orderFlowDay(9, 1), orderFlowDay(10, 3)))

	require.ErrorIs(t, err, domains.ErrContractOrderFlowRefused)
	assert.Contains(t, err.Error(), "訂單流一次至多涵蓋 31 天")
}

func TestARememberedDayIsNotFetchedAgainUnlessThePriceStepChanges(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
		DoAndReturn(archiveHolding(buyerTrade(orderFlowDay(10, 12), 84160))).Times(2)
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))

	for range 2 {
		_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
			t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
		require.NoError(t, err)
	}
	first := (*fixture.footprintsSeen)[0]
	second := (*fixture.footprintsSeen)[1]
	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 25), requestDto)

	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 84150.0, fixture.lastFootprints()[0].PointOfControl)
}

func TestTodayOnlyAsksForTheMinutesNotYetRemembered(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(at14(9, 10), at14(9, 14), at14(9, 19))
	gomock.InOrder(
		fixture.tradeHistoryProxy.EXPECT().
			VisitTradesBetween(gomock.Any(), "BTCUSDT", orderFlowDay(10, 14), at14(9, 15), gomock.Any()).
			DoAndReturn(liveHolding(buyerTrade(at14(9, 10), 100))),
		fixture.tradeHistoryProxy.EXPECT().
			VisitTradesBetween(gomock.Any(), "BTCUSDT", at14(9, 15), at14(9, 20), gomock.Any()).
			DoAndReturn(liveHolding(buyerTrade(at14(9, 19), 200))),
	)

	_, firstError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("5m", at14(9, 10), at14(9, 15)))
	fixture.setNow(at14(9, 20).Add(5 * time.Second))
	_, secondError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("5m", at14(9, 10), at14(9, 20)))

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	footprints := fixture.lastFootprints()
	require.Len(t, footprints, 2)
	assert.Equal(t, 100.0, footprints[0].PointOfControl)
	assert.Equal(t, 200.0, footprints[1].PointOfControl)
}

func TestOrderFlowNotGatheredInTimeIsNotReadyAndKeepsItsWholeMinutes(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(at14(9, 14))
	gomock.InOrder(
		fixture.tradeHistoryProxy.EXPECT().
			VisitTradesBetween(gomock.Any(), "BTCUSDT", orderFlowDay(10, 14), at14(9, 15), gomock.Any()).
			DoAndReturn(func(executionContext context.Context, _ string, _ time.Time, _ time.Time, visit tradeVisit) error {
				visit(buyerTrade(at14(0, 0).Add(30*time.Second), 100))
				visit(buyerTrade(at14(0, 5).Add(10*time.Second), 100))
				<-executionContext.Done()

				return executionContext.Err()
			}),
		// 00:05 had only begun when time ran out, so it is asked for again.
		fixture.tradeHistoryProxy.EXPECT().
			VisitTradesBetween(gomock.Any(), "BTCUSDT", at14(0, 5), at14(9, 15), gomock.Any()).
			Return(nil),
	)
	requestDto := orderFlowRequest("1m", at14(9, 14), orderFlowToday)
	requestDto.OrderFlowPreparationAllowance = 50 * time.Millisecond

	_, notReadyError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
	_, laterError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	require.ErrorIs(t, notReadyError, domains.ErrContractOrderFlowNotReady)
	assert.Contains(t, notReadyError.Error(), "訂單流還沒備齊：50ms 內沒能備齊")
	assert.NoError(t, laterError)
}

func TestACallerThatStopsWaitingIsToldSoNotThatOrderFlowIsLate(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(at14(9, 14))
	callerContext, stopCalling := context.WithCancel(t.Context())
	fixture.tradeHistoryProxy.EXPECT().VisitTradesBetween(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, _ string, _ time.Time, _ time.Time, _ tradeVisit) error {
			stopCalling()
			<-executionContext.Done()

			return executionContext.Err()
		})
	requestDto := orderFlowRequest("1m", at14(9, 14), orderFlowToday)
	requestDto.OrderFlowPreparationAllowance = time.Minute

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		callerContext, indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, domains.ErrContractOrderFlowNotReady)
}

func TestTwoRunsNeedingTheSameDayFetchItOnce(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ time.Time, visit tradeVisit) (bool, error) {
			time.Sleep(50 * time.Millisecond)
			visit(buyerTrade(orderFlowDay(10, 12), 100))

			return true, nil
		}).Times(1)
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))

	waitGroup := sync.WaitGroup{}
	errs := make([]error, 2)
	for runIndex := range 2 {
		waitGroup.Go(func() {
			_, errs[runIndex] = fixture.indicatorCalculationApplication.CalculateContractIndicator(
				context.Background(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
		})
	}
	waitGroup.Wait()

	assert.NoError(t, errs[0])
	assert.NoError(t, errs[1])
}

func TestTheLeastRecentlyUsedDayIsForgottenWhenMemoryIsFull(t *testing.T) {
	// Room for two days holding one traded minute each, not three.
	fixture := newOrderFlowUnderTest(t, 450)
	first, second, third := orderFlowDay(10, 10), orderFlowDay(10, 11), orderFlowDay(10, 12)
	fixture.kCandleContractRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cutoff time.Time, _ int) ([]entities.KCandleContract, error) {
			return storedContractCandlesAt(cutoff.Add(-time.Hour)), nil
		}).AnyTimes()
	fetches := map[time.Time]int{}
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, day time.Time, visit tradeVisit) (bool, error) {
			fetches[day]++
			visit(buyerTrade(day, 100))

			return true, nil
		}).AnyTimes()

	// The first day is used again just before the third arrives, so the second is the one forgotten.
	for _, day := range []time.Time{first, second, first, third, first, second} {
		_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
			t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), orderFlowRequest("1h", day, day.Add(time.Hour)))
		require.NoError(t, err)
	}

	assert.Equal(t, map[time.Time]int{first: 1, second: 2, third: 1}, fetches)
}

func TestADayLargerThanTheWholeMemoryIsNeverKept(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 100)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
		DoAndReturn(archiveHolding(buyerTrade(orderFlowDay(10, 12), 100))).Times(2)
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))

	for range 2 {
		_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
			t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
		require.NoError(t, err)
	}
}

func at14(hour int, minute int) time.Time {
	return time.Date(2026, 10, 14, hour, minute, 0, 0, time.UTC)
}

func TestADayWhoseArchiveBreaksHalfwayIsNotRemembered(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	gomock.InOrder(
		fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, _ time.Time, visit tradeVisit) (bool, error) {
				visit(buyerTrade(orderFlowDay(10, 12), 100))

				return false, errors.New("connection reset halfway through the file")
			}),
		fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
			DoAndReturn(archiveHolding(buyerTrade(orderFlowDay(10, 12), 200))),
	)
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))

	_, brokenError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
	_, laterError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	require.ErrorIs(t, brokenError, domains.ErrContractOrderFlowUnavailable)
	require.NoError(t, laterError)
	assert.Equal(t, 200.0, fixture.lastFootprints()[0].PointOfControl, "第二次整份重拿，半份不留")
}

func TestARunWaitingBehindAnotherForTheSameDayStillStopsWhenItsOwnTimeRunsOut(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), "BTCUSDT", orderFlowDay(10, 12), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ time.Time, _ tradeVisit) (bool, error) {
			close(firstStarted)
			<-releaseFirst

			return true, nil
		}).Times(1)
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = fixture.indicatorCalculationApplication.CalculateContractIndicator(
			context.Background(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)
	}()
	<-firstStarted

	boundedRequest := requestDto
	boundedRequest.OrderFlowPreparationAllowance = 50 * time.Millisecond
	startedAt := time.Now()
	_, waitingError := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), boundedRequest)
	close(releaseFirst)
	<-firstDone

	require.ErrorIs(t, waitingError, domains.ErrContractOrderFlowNotReady)
	assert.Less(t, time.Since(startedAt), 2*time.Second)
}

func TestTheLastMinuteIsNotReadBeforeItsTradesHaveSettled(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	// Right on the close of 09:14: its last instant's trades may not be on the live venue yet.
	fixture.setNow(at14(9, 15))
	fixture.storesContractCandles(at14(9, 14))
	requestDto := orderFlowRequest("1m", at14(9, 14), at14(9, 15))
	requestDto.OrderFlowPreparationAllowance = 50 * time.Millisecond

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	// The trade history mock expects nothing: being asked before the trades settled fails the test.
	require.ErrorIs(t, err, domains.ErrContractOrderFlowNotReady)
}

func TestAVenueConnectionTimingOutIsTheVenueFailingNotTheAllowance(t *testing.T) {
	fixture := newOrderFlowUnderTest(t, 1<<20)
	fixture.storesContractCandles(orderFlowDay(10, 12))
	fixture.tradeHistoryProxy.EXPECT().VisitDailyTrades(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, fmt.Errorf("read trade archive: %w", context.DeadlineExceeded))
	requestDto := orderFlowRequest("1h", orderFlowDay(10, 12), orderFlowDay(10, 12).Add(time.Hour))
	requestDto.OrderFlowPreparationAllowance = time.Minute

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carryingOrderFlow(t, 10), requestDto)

	require.ErrorIs(t, err, domains.ErrContractOrderFlowUnavailable)
	assert.NotErrorIs(t, err, domains.ErrContractOrderFlowNotReady)
}
