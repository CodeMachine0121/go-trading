package application_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"go.uber.org/mock/gomock"
)

const (
	screeningAdministratorID = uint(7)
	screeningProfileID       = uint(4)
	screeningRunID           = uint(9)
	screeningScriptID        = uint(108)
	screeningReplica         = "replica-a"
)

var (
	screeningMoment    = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	screeningPeriodEnd = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	screeningDay       = 24 * time.Hour
)

type symbolScreeningUnderTest struct {
	application          *application.SymbolScreeningApplication
	profileRepository    *mocks.MockISymbolScreeningProfileRepository
	runRepository        *mocks.MockISymbolScreeningRunRepository
	candidateRepository  *mocks.MockISymbolScreeningCandidateRepository
	kCandleRepository    *mocks.MockISymbolScreeningKCandleRepository
	settlementRepository *mocks.MockISymbolScreeningFundingRateSettlementRepository
	strategyScriptRepo   *mocks.MockIStrategyScriptRepository
	marketDataProxy      *mocks.MockIContractMarketDataProxy
	symbolLookupProxy    *mocks.MockIContractSymbolLookupProxy
	fundingRateProxy     *mocks.MockIContractFundingRateProxy
	marginTierProxy      *mocks.MockIContractMaintenanceMarginTierProxy
	contractScriptProxy  *mocks.MockIContractIndicatorScriptProxy
	savedCandidatesMutex *sync.Mutex
	savedCandidates      map[string]entities.SymbolScreeningCandidate
	runWritesMutex       *sync.Mutex
	runWrites            *[]entities.SymbolScreeningRun
	runEnded             chan entities.SymbolScreeningRun
	unfinishedCancelled  chan uint
}

func newSymbolScreeningUnderTest(t *testing.T, timeAllowance time.Duration) symbolScreeningUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(screeningMoment).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()

	underTest := symbolScreeningUnderTest{
		profileRepository:    mocks.NewMockISymbolScreeningProfileRepository(controller),
		runRepository:        mocks.NewMockISymbolScreeningRunRepository(controller),
		candidateRepository:  mocks.NewMockISymbolScreeningCandidateRepository(controller),
		kCandleRepository:    mocks.NewMockISymbolScreeningKCandleRepository(controller),
		settlementRepository: mocks.NewMockISymbolScreeningFundingRateSettlementRepository(controller),
		strategyScriptRepo:   mocks.NewMockIStrategyScriptRepository(controller),
		marketDataProxy:      mocks.NewMockIContractMarketDataProxy(controller),
		symbolLookupProxy:    mocks.NewMockIContractSymbolLookupProxy(controller),
		fundingRateProxy:     mocks.NewMockIContractFundingRateProxy(controller),
		marginTierProxy:      mocks.NewMockIContractMaintenanceMarginTierProxy(controller),
		contractScriptProxy:  mocks.NewMockIContractIndicatorScriptProxy(controller),
		savedCandidatesMutex: &sync.Mutex{},
		savedCandidates:      map[string]entities.SymbolScreeningCandidate{},
		runWritesMutex:       &sync.Mutex{},
		runWrites:            &[]entities.SymbolScreeningRun{},
		runEnded:             make(chan entities.SymbolScreeningRun, 1),
		unfinishedCancelled:  make(chan uint, 1),
	}

	contractBacktestService := service.NewContractBacktestService(
		mocks.NewMockIKCandleContractRepository(controller), mocks.NewMockIContractFundingRateSettlementRepository(controller),
		mocks.NewMockIContractPositionStatisticRepository(controller), mocks.NewMockIContractTradingSymbolRepository(controller),
		mocks.NewMockIContractMaintenanceMarginTierRepository(controller), underTest.contractScriptProxy,
		noOrderFlowService(t), clockProxy, 50000, 0)
	marketDataService := service.NewSymbolScreeningMarketDataService(
		underTest.kCandleRepository, underTest.settlementRepository, underTest.marketDataProxy,
		underTest.symbolLookupProxy, underTest.fundingRateProxy, underTest.marginTierProxy, clockProxy)
	runService := service.NewSymbolScreeningRunService(
		underTest.runRepository, underTest.candidateRepository, transactionRepository, clockProxy,
		marketDataService, contractBacktestService, screeningReplica, 20, timeAllowance, 0)
	underTest.application = application.NewSymbolScreeningApplication(
		service.NewSymbolScreeningProfileService(underTest.profileRepository, underTest.runRepository,
			service.NewStrategyScriptService(underTest.strategyScriptRepo,
				mocks.NewMockIPublishedStrategyScriptRepository(controller))),
		runService, marketDataService, 90*screeningDay, 30*screeningDay)

	return underTest
}

func anOwnedContractSignalScript() entities.StrategyScript {
	return entities.StrategyScript{
		ID: screeningScriptID, OwnerID: screeningAdministratorID, Name: "V3", Script: "the script",
		ResultType: "signal", MarketDataKind: string(vo.MarketDataKindContractKCandle),
	}
}

func aStoredScreeningProfile() entities.SymbolScreeningProfile {
	return entities.SymbolScreeningProfile{
		ID: screeningProfileID, OwnerID: screeningAdministratorID, Name: "趨勢 V3", StrategyScriptID: screeningScriptID,
		ParameterValues: []dto.StrategyScriptParameterValueDto{}, AggregationInterval: "4h", TradingMode: "longOnly",
		Leverage: decimal.NewFromInt(1), EntryCostPercentage: decimal.RequireFromString("0.05"),
		SlippagePercentage: decimal.RequireFromString("0.05"), HistoryDays: 730, SegmentCount: 3, MinimumTradeCount: 6,
		MinimumAverageDailyQuoteVolume: decimal.NewFromInt(20_000_000), BaselineSymbols: []string{},
	}
}

func (underTest symbolScreeningUnderTest) theScriptIsOwned() {
	underTest.strategyScriptRepo.EXPECT().FindOne(gomock.Any(), screeningScriptID).
		Return(anOwnedContractSignalScript(), nil).AnyTimes()
}

// aQueuedRun screens daily bars over 180 days in three segments, so each segment replays 60 bars.
func aQueuedRun(baselineSymbols ...string) entities.SymbolScreeningRun {
	return entities.SymbolScreeningRun{
		ID: screeningRunID, OwnerID: screeningAdministratorID, Status: "queued", CandidateCount: 2,
		Snapshot: dto.SymbolScreeningSnapshotDto{
			ProfileName: "趨勢 V3", Script: "the script", ResultType: "signal", AggregationInterval: "1d",
			TradingMode: "longOnly", Leverage: decimal.NewFromInt(1), EntryCostPercentage: decimal.RequireFromString("0.05"),
			SlippagePercentage: decimal.RequireFromString("0.05"), HistoryDays: 180, SegmentCount: 3, MinimumTradeCount: 6,
			MinimumAverageDailyQuoteVolume: decimal.NewFromInt(20_000_000), BaselineSymbols: baselineSymbols,
		},
	}
}

func screeningCandidates(symbols ...string) []entities.SymbolScreeningCandidate {
	candidates := []entities.SymbolScreeningCandidate{}
	for index, symbol := range symbols {
		candidates = append(candidates, entities.SymbolScreeningCandidate{
			ID: uint(index + 1), RunID: screeningRunID, Position: index + 1, Symbol: symbol, Status: "waiting",
		})
	}

	return candidates
}

// claimsTheRun hands the queued run to the worker and records every write the runner makes to it.
func (underTest symbolScreeningUnderTest) claimsTheRun(run entities.SymbolScreeningRun, symbols ...string) {
	underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(run, true, nil)
	underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
	underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).
		Return(screeningCandidates(symbols...), nil)
	underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, candidate entities.SymbolScreeningCandidate) error {
			underTest.savedCandidatesMutex.Lock()
			defer underTest.savedCandidatesMutex.Unlock()
			underTest.savedCandidates[candidate.Symbol] = candidate

			return nil
		}).AnyTimes()
	underTest.candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), screeningRunID).
		DoAndReturn(func(_ context.Context, runID uint) error {
			underTest.unfinishedCancelled <- runID
			return nil
		}).AnyTimes()
}

// runWritesSucceed accepts every write while the run is still running, signalling the ending.
func (underTest symbolScreeningUnderTest) runWritesSucceed() {
	underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, run entities.SymbolScreeningRun) (bool, error) {
			underTest.runWritesMutex.Lock()
			*underTest.runWrites = append(*underTest.runWrites, run)
			underTest.runWritesMutex.Unlock()
			if run.Status != "running" {
				underTest.runEnded <- run
			}

			return true, nil
		}).AnyTimes()
}

func (underTest symbolScreeningUnderTest) candidateSaved(symbol string) entities.SymbolScreeningCandidate {
	underTest.savedCandidatesMutex.Lock()
	defer underTest.savedCandidatesMutex.Unlock()

	return underTest.savedCandidates[symbol]
}

func (underTest symbolScreeningUnderTest) endingOf(t *testing.T) entities.SymbolScreeningRun {
	t.Helper()
	select {
	case ending := <-underTest.runEnded:
		return ending
	case <-time.After(10 * time.Second):
		t.Fatal("the run never ended")
		return entities.SymbolScreeningRun{}
	}
}

var screeningSpecification = vo.ContractTradingSpecificationVo{
	Symbol: "LINKUSDT", TickSize: decimal.RequireFromString("0.0001"), QuantityStep: decimal.RequireFromString("0.01"),
	MinimumQuantity: decimal.RequireFromString("0.01"), MinimumNotional: decimal.NewFromInt(5),
	MaintenanceMarginRate: decimal.RequireFromString("0.005"), LiquidationFeeRate: decimal.RequireFromString("0.005"),
}

// isTrading lists the symbol and answers its daily bars, each closing dailyChange higher or lower than the last.
func (underTest symbolScreeningUnderTest) isTrading(symbol string, dailyChange float64) {
	specification := screeningSpecification
	specification.Symbol = symbol
	underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), symbol).
		Return(vo.ContractSymbolListingVo{IsListed: true, IsKnownToVenue: true, Specification: specification}, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), symbol, "1d", gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), symbol, gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.settlementRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), symbol, gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchAggregatedKCandles(gomock.Any(), gomock.Cond(
		func(window vo.AggregatedKCandleFetchWindowVo) bool { return window.Symbol == symbol })).
		DoAndReturn(func(_ context.Context, window vo.AggregatedKCandleFetchWindowVo) ([]vo.ContractMarketKCandleVo, error) {
			return dailyBars(symbol, window.StartTime, dailyChange), nil
		}).AnyTimes()
}

func dailyBars(symbol string, firstOpen time.Time, dailyChange float64) []vo.ContractMarketKCandleVo {
	bars := []vo.ContractMarketKCandleVo{}
	price := decimal.NewFromInt(100)
	for openTime := firstOpen.Add(-400 * screeningDay); openTime.Before(screeningPeriodEnd); openTime = openTime.Add(screeningDay) {
		next := price.Mul(decimal.NewFromFloat(1 + dailyChange))
		bars = append(bars, vo.ContractMarketKCandleVo{
			Symbol: symbol, OpenTime: openTime, Open: price, High: decimal.Max(price, next), Low: decimal.Min(price, next),
			Close: next, Volume: decimal.NewFromInt(1_000_000), QuoteVolume: decimal.NewFromInt(100_000_000),
			MarkOpen: decimal.NewNullDecimal(price), MarkHigh: decimal.NewNullDecimal(decimal.Max(price, next)),
			MarkLow: decimal.NewNullDecimal(decimal.Min(price, next)), MarkClose: decimal.NewNullDecimal(next),
		})
		price = next
	}

	return bars
}

// scriptTradesEveryTenBars buys on every tenth bar and sells five bars later, so each sixty-bar segment closes several trades.
func (underTest symbolScreeningUnderTest) scriptTradesEveryTenBars() {
	underTest.contractScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
			_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
		) ([]map[string]vo.IndicatorValueVo, error) {
			signals := make([]vo.SignalVo, 0, len(bars))
			for index := range bars {
				signal := vo.SignalHold
				switch index % 10 {
				case 0:
					signal = vo.SignalBuy
				case 5:
					signal = vo.SignalSell
				}
				signals = append(signals, signal)
			}

			return signalsSaying(signals...), nil
		}).AnyTimes()
}
