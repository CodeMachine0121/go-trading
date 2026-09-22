package service_test

import (
	"context"
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

const contractHistoryCeilingDays = 3650

// reportedContractCandle is one contract candle as the proxy hands it over, with its
// mark price present.
func reportedContractCandle(openTime time.Time) vo.ContractMarketKCandleVo {
	return vo.ContractMarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
		TradeCount:          7,
		MarkOpen:            decimal.NewNullDecimal(decimal.RequireFromString("101")),
		MarkHigh:            decimal.NewNullDecimal(decimal.RequireFromString("121")),
		MarkLow:             decimal.NewNullDecimal(decimal.RequireFromString("91")),
		MarkClose:           decimal.NewNullDecimal(decimal.RequireFromString("111")),
	}
}

// reportedContractCandleWithoutMarkPrice is the same minute as it arrives when the
// venue answered the traded question but not the mark price one.
func reportedContractCandleWithoutMarkPrice(openTime time.Time) vo.ContractMarketKCandleVo {
	contractCandle := reportedContractCandle(openTime)
	contractCandle.MarkOpen = decimal.NullDecimal{}
	contractCandle.MarkHigh = decimal.NullDecimal{}
	contractCandle.MarkLow = decimal.NullDecimal{}
	contractCandle.MarkClose = decimal.NullDecimal{}

	return contractCandle
}

type contractIngestionUnderTest struct {
	service                   *service.ContractKCandleIngestionService
	kCandleContractRepository *mocks.MockIKCandleContractRepository
	syncRunRepository         *mocks.MockIKCandleContractHistorySyncRunRepository
	symbolRepository          *mocks.MockIContractTradingSymbolRepository
	marketDataProxy           *mocks.MockIContractMarketDataProxy
	clockProxy                *mocks.MockIClockProxy
}

func newContractIngestionUnderTest(t *testing.T, currentTime time.Time) contractIngestionUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(mockController)
	syncRunRepository := mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()

	return contractIngestionUnderTest{
		service: service.NewContractKCandleIngestionService(
			kCandleContractRepository, syncRunRepository, symbolRepository,
			marketDataProxy, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
			roundCandleCount, lookback),
		kCandleContractRepository: kCandleContractRepository,
		syncRunRepository:         syncRunRepository,
		symbolRepository:          symbolRepository,
		marketDataProxy:           marketDataProxy,
		clockProxy:                clockProxy,
	}
}

func (underTest contractIngestionUnderTest) watching(symbols ...string) {
	watchedSymbols := make([]entities.ContractTradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols,
			entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true})
	}

	underTest.symbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(watchedSymbols, nil).AnyTimes()
}

func (underTest contractIngestionUnderTest) registered(symbol string) {
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), symbol).Return(
		entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true}, true, nil).AnyTimes()
}

func (underTest contractIngestionUnderTest) storingEveryCandle() {
	underTest.kCandleContractRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, nil).AnyTimes()
}

func TestContractRoundStoresTheCandlesTheVenueAnsweredWith(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{
			reportedContractCandle(ingestionAt(9, 4, 0)),
			reportedContractCandle(ingestionAt(9, 5, 0)),
			reportedContractCandle(ingestionAt(9, 6, 0)),
		}, nil)
	underTest.kCandleContractRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, nil).Times(3)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, "BTCUSDT", report.SymbolReports[0].Symbol)
	assert.Equal(t, 3, report.SymbolReports[0].StoredCount)
	assert.Equal(t, 0, report.SymbolReports[0].SkippedCount)
}

func TestContractRoundSkipsTheMinuteWhoseMarkPriceNeverArrived(t *testing.T) {
	// A contract candle without its mark price is not one, so it goes down the same
	// path a candle breaking any other rule does: skipped, named, and the rest stored.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{
			reportedContractCandle(ingestionAt(9, 5, 0)),
			reportedContractCandleWithoutMarkPrice(ingestionAt(9, 6, 0)),
		}, nil)
	underTest.kCandleContractRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, nil).Times(1)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Equal(t, 1, report.SymbolReports[0].SkippedCount)
	require.Len(t, report.SymbolReports[0].SkippedKCandles, 1)
	assert.Equal(t, ingestionAt(9, 6, 0), report.SymbolReports[0].SkippedKCandles[0].OpenTime)
	assert.Contains(t, report.SymbolReports[0].SkippedKCandles[0].Reason, "標記價格不得留白")
}

func TestContractRoundStoresNothingWhenTheVenueWillNotAnswer(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "market source unreachable")
}

func TestContractRoundDoesNotStoreTheMinuteStillRunning(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{
			reportedContractCandle(ingestionAt(9, 6, 0)),
			reportedContractCandle(ingestionAt(9, 7, 0)),
		}, nil)

	storedOpenTimes := make([]time.Time, 0)
	underTest.kCandleContractRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, storedCandle entities.KCandleContract,
		) (entities.KCandleContract, error) {
			storedOpenTimes = append(storedOpenTimes, storedCandle.OpenTime)

			return storedCandle, nil
		}).AnyTimes()

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Equal(t, []time.Time{ingestionAt(9, 6, 0)}, storedOpenTimes)
}

func TestContractRoundKeepsFetchingThroughTheNightAndNeverPresumesAHoliday(t *testing.T) {
	// A perpetual contract has no session and therefore no day off. An hour the venue
	// had nothing for is a quiet hour, and the next round asks again all the same.
	underTest := newContractIngestionUnderTest(t, ingestionAt(3, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.ContractMarketKCandleVo{}, nil).Times(2)

	firstReport, firstError := underTest.service.RunScheduledRound(t.Context())
	secondReport, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.Equal(t, 0, firstReport.SymbolReports[0].StoredCount)
	assert.Equal(t, 0, secondReport.SymbolReports[0].StoredCount)
	assert.Empty(t, secondReport.SymbolReports[0].FetchFailureReason)
}

func TestContractRoundLeavesOneContractsFailureToItself(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT", "ETHUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, window vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			if window.Symbol == "BTCUSDT" {
				return nil, sourceUnreachable
			}
			healthyCandle := reportedContractCandle(ingestionAt(9, 6, 0))
			healthyCandle.Symbol = window.Symbol

			return []vo.ContractMarketKCandleVo{healthyCandle}, nil
		}).AnyTimes()
	underTest.storingEveryCandle()

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 2)
	reportBySymbol := make(map[string]dto.KCandleSymbolIngestionReportDto, 2)
	for _, symbolReport := range report.SymbolReports {
		reportBySymbol[symbolReport.Symbol] = symbolReport
	}
	assert.NotEmpty(t, reportBySymbol["BTCUSDT"].FetchFailureReason)
	assert.Equal(t, 1, reportBySymbol["ETHUSDT"].StoredCount)
}

func TestContractBackfillResumesAfterTheNewestStoredCandle(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).Return(
		[]entities.KCandleContract{{Symbol: "BTCUSDT", OpenTime: ingestionAt(9, 0, 0)}}, nil)

	askedWindow := vo.KCandleFetchWindowVo{}
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, window vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			askedWindow = window

			return []vo.ContractMarketKCandleVo{}, nil
		})

	_, runError := underTest.service.RunBackfill(t.Context())

	require.NoError(t, runError)
	assert.Equal(t, ingestionAt(9, 1, 0), askedWindow.StartTime)
	assert.Equal(t, ingestionAt(9, 6, 0), askedWindow.EndTime)
}

func TestContractBackfillForOneContractRefusesAnUnregisteredName(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{}, false, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "BTCUSDT")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNotRegistered)
}

func TestContractBackfillForOneContractRefusesABlankName(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "   ")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNamed)
}

// contractHistorySyncRuns collects every write the background walk makes, so a case
// can wait for the run to be closed off rather than guessing how long it takes.
type contractHistorySyncRuns struct {
	ended chan entities.KCandleContractHistorySyncRun
}

func (runs *contractHistorySyncRuns) record(
	syncRun entities.KCandleContractHistorySyncRun,
) (entities.KCandleContractHistorySyncRun, error) {
	if syncRun.ID == 0 {
		syncRun.ID = 1
	}
	if syncRun.FinishedAt != nil {
		select {
		case runs.ended <- syncRun:
		default:
		}
	}

	return syncRun, nil
}

func (runs *contractHistorySyncRuns) awaitEnding(t *testing.T) entities.KCandleContractHistorySyncRun {
	t.Helper()

	select {
	case endedRun := <-runs.ended:
		return endedRun
	case <-time.After(5 * time.Second):
		t.Fatal("合約歷史同步沒有收尾")

		return entities.KCandleContractHistorySyncRun{}
	}
}

func (underTest contractIngestionUnderTest) recordsEveryContractSyncRun() *contractHistorySyncRuns {
	runs := &contractHistorySyncRuns{ended: make(chan entities.KCandleContractHistorySyncRun, 1)}
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleContractHistorySyncRun,
		) (entities.KCandleContractHistorySyncRun, error) {
			return runs.record(syncRun)
		}).AnyTimes()

	return runs
}

func TestContractHistorySyncAnswersBeforeItHasFetchedAnything(t *testing.T) {
	// Every minute of this stretch takes two questions to the venue, so a long one is
	// twice the round trips the spot side makes. The run is the answer.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	letGo := make(chan struct{})
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			<-letGo

			return []vo.ContractMarketKCandleVo{}, nil
		}).AnyTimes()
	underTest.kCandleContractRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	startedRun, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	assert.NotZero(t, startedRun.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncRunning), startedRun.Status)
	close(letGo)
	runs.awaitEnding(t)
}

func TestContractHistorySyncDoesNotAskAboutADayItAlreadyHoldsWhole(t *testing.T) {
	// A stored contract candle is a complete one, so counting candles is enough to
	// know both halves of every minute are there.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(100000, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Equal(t, 0, endedRun.StoredCount)
}

func TestContractHistorySyncAsksAgainAboutADayItOnlyHoldsHalfOf(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(1, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{reportedContractCandle(ingestionAt(9, 5, 0))}, nil).AnyTimes()
	underTest.kCandleContractRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(1, nil).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Positive(t, endedRun.StoredCount)
}

func TestContractHistorySyncSucceedsWhenTheContractDidNotYetExist(t *testing.T) {
	// The venue answering with nothing for a stretch is something this run found out,
	// not something it did wrong.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.ContractMarketKCandleVo{}, nil).AnyTimes()
	underTest.kCandleContractRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Equal(t, 0, endedRun.StoredCount)
	assert.Empty(t, endedRun.FailureReason)
}

func TestContractHistorySyncWritesWhatTheVenueRefusedWithoutFailingTheRun(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Contains(t, endedRun.FetchFailureReason, "market source unreachable")
	assert.Empty(t, endedRun.FailureReason)
}

func TestContractHistorySyncFailsTheRunWhenStorageBreaks(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, sourceUnreachable).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
	assert.NotEmpty(t, endedRun.FailureReason)
}

func TestContractHistorySyncRefusesALookbackThatIsNotAStretch(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))

	_, zeroError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 0},
		contractHistoryCeilingDays)
	_, overCeilingError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 99999},
		contractHistoryCeilingDays)

	assert.ErrorIs(t, zeroError, domains.ErrKCandleHistoryLookback)
	assert.ErrorIs(t, overCeilingError, domains.ErrKCandleHistoryLookback)
	assert.Contains(t, overCeilingError.Error(), "3650")
}

func TestContractHistorySyncRefusesASecondRunOnTheSameContract(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(
		entities.KCandleContractHistorySyncRun{},
		domains.KCandleHistorySyncInProgress("BTCUSDT"))

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	assert.ErrorIs(t, startError, domains.ErrKCandleHistorySyncInProgress)
}

func TestContractHistorySyncRunIsReadableByItsNumber(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
		entities.KCandleContractHistorySyncRun{
			ID: 7, Symbol: "BTCUSDT", Status: string(vo.KCandleHistorySyncRunning),
			StartedAt: ingestionAt(9, 0, 0),
		}, true, nil)
	underTest.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(8)).Return(
		entities.KCandleContractHistorySyncRun{}, false, nil)

	foundRun, foundError := underTest.service.GetHistorySyncRun(t.Context(), 7)
	_, missingError := underTest.service.GetHistorySyncRun(t.Context(), 8)

	require.NoError(t, foundError)
	assert.Equal(t, uint(7), foundRun.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncRunning), foundRun.Status)
	assert.ErrorIs(t, missingError, service.ErrKCandleHistorySyncRunNotFound)
}

func TestContractHistorySyncSweepsTheRunsARestartCutOff(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncRunRepository.EXPECT().
		FailAllRunning(gomock.Any(), "interrupted by restart", ingestionAt(9, 7, 30)).
		Return(2, nil)

	sweptCount, sweepError := underTest.service.FailInterruptedHistorySyncs(t.Context())

	require.NoError(t, sweepError)
	assert.Equal(t, 2, sweptCount)
}

func TestContractRoundRefusesRulesItCannotSettle(t *testing.T) {
	// A run that cannot work whatever it is pointed at says so before it has read a
	// watchlist or touched a source.
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 30)).AnyTimes()
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	symbolRepository.EXPECT().FindWatched(gomock.Any()).Times(0)
	brokenService := service.NewContractKCandleIngestionService(
		mocks.NewMockIKCandleContractRepository(mockController),
		mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController),
		symbolRepository,
		mocks.NewMockIContractMarketDataProxy(mockController),
		clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
		0, lookback)

	_, roundError := brokenService.RunScheduledRound(t.Context())
	_, backfillError := brokenService.RunBackfill(t.Context())
	_, catchUpError := brokenService.RunBackfillFor(t.Context(), "BTCUSDT")

	assert.Error(t, roundError)
	assert.Error(t, backfillError)
	assert.Error(t, catchUpError)
}

func TestContractRoundReportsAWatchlistItCannotRead(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return(nil, sourceUnreachable).Times(2)

	_, roundError := underTest.service.RunScheduledRound(t.Context())
	_, backfillError := underTest.service.RunBackfill(t.Context())

	assert.ErrorIs(t, roundError, sourceUnreachable)
	assert.ErrorIs(t, backfillError, sourceUnreachable)
}

func TestContractBackfillReportsStorageItCannotReadTheGapFrom(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return(nil, sourceUnreachable)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	report, runError := underTest.service.RunBackfill(t.Context())

	require.NoError(t, runError)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "market source unreachable")
}

func TestContractBackfillAsksForNothingWhenTheGapIsAlreadyClosed(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).Return(
		[]entities.KCandleContract{{Symbol: "BTCUSDT", OpenTime: ingestionAt(9, 6, 0)}}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	report, runError := underTest.service.RunBackfill(t.Context())

	require.NoError(t, runError)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
}

func TestContractRoundNotesACandleStorageWouldNotTake(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{reportedContractCandle(ingestionAt(9, 6, 0))}, nil)
	underTest.kCandleContractRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, sourceUnreachable)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Equal(t, 1, report.SymbolReports[0].SkippedCount)
}

func TestContractRoundStopsNamingSkippedCandlesOnceThereAreTooMany(t *testing.T) {
	// The count still answers "how bad is it" after the list stops growing, so a venue
	// answering with rubbish cannot turn the report into something nobody can open.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watching("BTCUSDT")

	brokenCandles := make([]vo.ContractMarketKCandleVo, 0, 250)
	for minute := range 250 {
		brokenCandles = append(brokenCandles, reportedContractCandleWithoutMarkPrice(
			ingestionAt(0, 0, 0).Add(time.Duration(minute)*time.Minute)))
	}
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(brokenCandles, nil)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	symbolReport := report.SymbolReports[0]
	assert.Equal(t, 250, symbolReport.SkippedCount)
	assert.Len(t, symbolReport.SkippedKCandles, 200)
	assert.True(t, symbolReport.SkippedKCandlesTruncated)
}

func TestContractHistorySyncFailsTheRunWhenStoringABatchBreaks(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(
		[]vo.ContractMarketKCandleVo{reportedContractCandle(ingestionAt(9, 5, 0))}, nil).AnyTimes()
	underTest.kCandleContractRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, sourceUnreachable).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
}

func TestContractHistorySyncReportsAContractItCannotLookUp(t *testing.T) {
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{}, false, sourceUnreachable)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	assert.ErrorIs(t, startError, sourceUnreachable)
}

func TestContractHistorySyncKeepsTryingToCloseOffARunStorageKeepsRefusing(t *testing.T) {
	// Losing the closing write leaves the row saying running for as long as this
	// process lives, so it is worth waiting on where a progress figure is not.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(100000, nil).AnyTimes()

	closingAttempts := make(chan struct{}, 8)
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleContractHistorySyncRun,
		) (entities.KCandleContractHistorySyncRun, error) {
			if syncRun.ID == 0 {
				syncRun.ID = 1

				return syncRun, nil
			}
			if syncRun.FinishedAt != nil {
				select {
				case closingAttempts <- struct{}{}:
				default:
				}
			}

			return entities.KCandleContractHistorySyncRun{}, sourceUnreachable
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	for range 3 {
		select {
		case <-closingAttempts:
		case <-time.After(5 * time.Second):
			t.Fatal("收尾的寫入沒有重試到約定的次數")
		}
	}
}

func TestContractHistorySyncClosesTheRunOffWhenTheWalkBreaksDown(t *testing.T) {
	// A panic out here has nothing above it to contain it, so it would take the API
	// and every other fetch in flight with it. The run is closed as failed on the way
	// out rather than left sitting at running until the next restart.
	underTest := newContractIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.registered("BTCUSDT")
	runs := underTest.recordsEveryContractSyncRun()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ time.Time, _ time.Time) (int, error) {
			panic("storage went away mid-walk")
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 2},
		contractHistoryCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
	assert.Contains(t, endedRun.FailureReason, "broke down")
}
