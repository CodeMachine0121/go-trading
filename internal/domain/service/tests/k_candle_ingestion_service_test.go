package service_test

import (
	"errors"
	"sync"
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

const (
	roundCandleCount = 5
	lookback         = 24 * time.Hour
)

var sourceUnreachable = errors.New("market source unreachable")

func ingestionAt(hour int, minute int, second int) time.Time {
	return time.Date(2026, 8, 30, hour, minute, second, 0, time.UTC)
}

// reportedKCandle is one candle as a source would hand it over. High and low are
// spelled out because they are the pair the K candle rules judge.
func reportedKCandle(openTime time.Time, high string, low string) vo.MarketKCandleVo {
	return vo.MarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString(high),
		Low:                 decimal.RequireFromString(low),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}
}

func validReportedKCandle(openTime time.Time) vo.MarketKCandleVo {
	return reportedKCandle(openTime, "120", "90")
}

// reportedFor is the same candle as it would arrive for another trading symbol.
func reportedFor(symbol string, openTime time.Time) vo.MarketKCandleVo {
	marketKCandle := validReportedKCandle(openTime)
	marketKCandle.Symbol = symbol

	return marketKCandle
}

type ingestionUnderTest struct {
	service           *service.KCandleIngestionService
	kCandleRepository *mocks.MockIKCandleRepository
	marketDataProxy   *mocks.MockIMarketDataProxy
}

func newIngestionUnderTest(t *testing.T, currentTime time.Time) ingestionUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	return ingestionUnderTest{
		service: service.NewKCandleIngestionService(
			kCandleRepository, marketDataProxy, clockProxy, roundCandleCount, lookback),
		kCandleRepository: kCandleRepository,
		marketDataProxy:   marketDataProxy,
	}
}

// savedOpenTimes records what actually reached storage, safely across the
// goroutines one round runs its symbols in.
type savedOpenTimes struct {
	mutex     sync.Mutex
	openTimes []time.Time
}

func (saved *savedOpenTimes) record(kCandle entities.KCandle) {
	saved.mutex.Lock()
	defer saved.mutex.Unlock()
	saved.openTimes = append(saved.openTimes, kCandle.OpenTime.UTC())
}

func (saved *savedOpenTimes) all() []time.Time {
	saved.mutex.Lock()
	defer saved.mutex.Unlock()
	return saved.openTimes
}

func (underTest ingestionUnderTest) acceptEverySave() *savedOpenTimes {
	saved := &savedOpenTimes{openTimes: []time.Time{}}
	underTest.kCandleRepository.EXPECT().Save(gomock.Any()).
		DoAndReturn(func(kCandle entities.KCandle) (entities.KCandle, error) {
			saved.record(kCandle)
			return kCandle, nil
		}).AnyTimes()

	return saved
}

func reportFor(t *testing.T, report dto.KCandleIngestionReportDto, symbol string) dto.KCandleSymbolIngestionReportDto {
	t.Helper()

	for _, symbolReport := range report.SymbolReports {
		if symbolReport.Symbol == symbol {
			return symbolReport
		}
	}
	t.Fatalf("no report for %s", symbol)

	return dto.KCandleSymbolIngestionReportDto{}
}

func TestScheduledRoundStoresTheNewestClosedCandles(t *testing.T) {
	testCases := []struct {
		name              string
		currentTime       time.Time
		reported          []vo.MarketKCandleVo
		expectedOpenTimes []time.Time
	}{
		{
			name:              "the newest closed candle is stored",
			currentTime:       ingestionAt(9, 7, 0),
			reported:          []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 0, 0))},
			expectedOpenTimes: []time.Time{ingestionAt(9, 0, 0)},
		},
		{
			name:        "the candle still running is left out",
			currentTime: ingestionAt(9, 9, 0),
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(9, 0, 0)),
				validReportedKCandle(ingestionAt(9, 5, 0)),
			},
			expectedOpenTimes: []time.Time{ingestionAt(9, 0, 0)},
		},
		{
			name:              "that same candle is stored once its interval has finished",
			currentTime:       ingestionAt(9, 11, 0),
			reported:          []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))},
			expectedOpenTimes: []time.Time{ingestionAt(9, 5, 0)},
		},
		{
			name:        "every candle of a full round is stored",
			currentTime: ingestionAt(9, 7, 0),
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(8, 40, 0)),
				validReportedKCandle(ingestionAt(8, 45, 0)),
				validReportedKCandle(ingestionAt(8, 50, 0)),
				validReportedKCandle(ingestionAt(8, 55, 0)),
				validReportedKCandle(ingestionAt(9, 0, 0)),
			},
			expectedOpenTimes: []time.Time{
				ingestionAt(8, 40, 0), ingestionAt(8, 45, 0), ingestionAt(8, 50, 0),
				ingestionAt(8, 55, 0), ingestionAt(9, 0, 0),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, testCase.currentTime)
			underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).Return(testCase.reported, nil)
			saved := underTest.acceptEverySave()

			report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})

			require.NoError(t, runError)
			assert.Equal(t, testCase.expectedOpenTimes, saved.all())
			assert.Equal(t, len(testCase.expectedOpenTimes), reportFor(t, report, "BTCUSDT").StoredCount)
		})
	}
}

func TestScheduledRoundAsksForTheNewestClosedCandlesBackwards(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(vo.NewKCandleFetchWindowVo(
		"BTCUSDT", ingestionAt(8, 40, 0), ingestionAt(9, 0, 0))).
		Return([]vo.MarketKCandleVo{}, nil)

	_, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})

	require.NoError(t, runError)
}

func TestScheduledRoundCoversEveryWatchedSymbol(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).
		DoAndReturn(func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			return []vo.MarketKCandleVo{reportedFor(window.Symbol, ingestionAt(9, 0, 0))}, nil
		}).Times(2)
	saved := underTest.acceptEverySave()

	report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT", "ETHUSDT"})

	require.NoError(t, runError)
	assert.Len(t, saved.all(), 2)
	assert.Equal(t, 1, reportFor(t, report, "BTCUSDT").StoredCount)
	assert.Equal(t, 1, reportFor(t, report, "ETHUSDT").StoredCount)
}

func TestScheduledRoundOnAnEmptyWatchlistDoesNothing(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))

	report, runError := underTest.service.RunScheduledRound([]string{})

	require.NoError(t, runError)
	assert.Empty(t, report.SymbolReports)
}

func TestOneSymbolFailingLeavesTheOthersAlone(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "BTCUSDT" })).
		Return(nil, sourceUnreachable)
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "ETHUSDT" })).
		Return([]vo.MarketKCandleVo{reportedFor("ETHUSDT", ingestionAt(9, 0, 0))}, nil)
	saved := underTest.acceptEverySave()

	report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT", "ETHUSDT"})

	require.NoError(t, runError)
	assert.Len(t, saved.all(), 1)
	assert.Equal(t, 0, reportFor(t, report, "BTCUSDT").StoredCount)
	assert.Contains(t, reportFor(t, report, "BTCUSDT").FetchFailureReason, "market source unreachable")
	assert.Equal(t, 1, reportFor(t, report, "ETHUSDT").StoredCount)
	assert.Empty(t, reportFor(t, report, "ETHUSDT").FetchFailureReason)
}

func TestEverySymbolFailingStillReportsRatherThanErroring(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).
		Return(nil, sourceUnreachable).Times(2)

	report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT", "ETHUSDT"})

	require.NoError(t, runError)
	assert.Contains(t, reportFor(t, report, "BTCUSDT").FetchFailureReason, "market source unreachable")
	assert.Contains(t, reportFor(t, report, "ETHUSDT").FetchFailureReason, "market source unreachable")
}

func TestACandleBreakingARuleIsSkippedOnItsOwn(t *testing.T) {
	testCases := []struct {
		name            string
		reported        []vo.MarketKCandleVo
		expectedStored  int
		expectedSkipped []dto.SkippedKCandleDto
	}{
		{
			name: "a high below the low",
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(8, 50, 0)),
				reportedKCandle(ingestionAt(8, 55, 0), "90", "100"),
				validReportedKCandle(ingestionAt(9, 0, 0)),
			},
			expectedStored: 2,
			expectedSkipped: []dto.SkippedKCandleDto{
				{OpenTime: ingestionAt(8, 55, 0), Reason: "最高價不得低於最低價"},
			},
		},
		{
			name: "an open time off the five minute mark",
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(8, 50, 0)),
				validReportedKCandle(ingestionAt(8, 53, 0)),
				validReportedKCandle(ingestionAt(9, 0, 0)),
			},
			expectedStored: 2,
			expectedSkipped: []dto.SkippedKCandleDto{
				{OpenTime: ingestionAt(8, 53, 0), Reason: "起始時間必須落在5分鐘刻度上"},
			},
		},
		{
			name: "every candle of the batch breaking a rule",
			reported: []vo.MarketKCandleVo{
				reportedKCandle(ingestionAt(8, 55, 0), "90", "100"),
				reportedKCandle(ingestionAt(9, 0, 0), "90", "100"),
			},
			expectedStored: 0,
			expectedSkipped: []dto.SkippedKCandleDto{
				{OpenTime: ingestionAt(8, 55, 0), Reason: "最高價不得低於最低價"},
				{OpenTime: ingestionAt(9, 0, 0), Reason: "最高價不得低於最低價"},
			},
		},
		{
			name: "a batch with nothing wrong with it",
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(8, 55, 0)),
				validReportedKCandle(ingestionAt(9, 0, 0)),
			},
			expectedStored:  2,
			expectedSkipped: []dto.SkippedKCandleDto{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
			underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).Return(testCase.reported, nil)
			underTest.acceptEverySave()

			report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})

			require.NoError(t, runError)
			symbolReport := reportFor(t, report, "BTCUSDT")
			assert.Equal(t, testCase.expectedStored, symbolReport.StoredCount)
			assert.Empty(t, symbolReport.FetchFailureReason)
			require.Len(t, symbolReport.SkippedKCandles, len(testCase.expectedSkipped))
			for index, expectedSkip := range testCase.expectedSkipped {
				assert.Equal(t, expectedSkip.OpenTime, symbolReport.SkippedKCandles[index].OpenTime)
				assert.Contains(t, symbolReport.SkippedKCandles[index].Reason, expectedSkip.Reason)
			}
		})
	}
}

func TestACandleThatCannotBeStoredIsSkippedRatherThanFailingTheSymbol(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).Return([]vo.MarketKCandleVo{
		validReportedKCandle(ingestionAt(8, 55, 0)),
		validReportedKCandle(ingestionAt(9, 0, 0)),
	}, nil)
	underTest.kCandleRepository.EXPECT().
		Save(gomock.Cond(func(kCandle entities.KCandle) bool { return kCandle.OpenTime.Equal(ingestionAt(8, 55, 0)) })).
		Return(entities.KCandle{}, errors.New("storage refused the write"))
	underTest.kCandleRepository.EXPECT().
		Save(gomock.Cond(func(kCandle entities.KCandle) bool { return kCandle.OpenTime.Equal(ingestionAt(9, 0, 0)) })).
		Return(entities.KCandle{}, nil)

	report, runError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})

	require.NoError(t, runError)
	symbolReport := reportFor(t, report, "BTCUSDT")
	assert.Equal(t, 1, symbolReport.StoredCount)
	assert.Empty(t, symbolReport.FetchFailureReason)
	require.Len(t, symbolReport.SkippedKCandles, 1)
	assert.Equal(t, ingestionAt(8, 55, 0), symbolReport.SkippedKCandles[0].OpenTime)
	assert.Contains(t, symbolReport.SkippedKCandles[0].Reason, "storage refused the write")
}

func TestBackfillAsksOnlyForTheGap(t *testing.T) {
	testCases := []struct {
		name              string
		stored            []entities.KCandle
		expectedStartTime time.Time
	}{
		{
			name:              "a gap inside the lookback starts after the stored candle",
			stored:            []entities.KCandle{{Symbol: "BTCUSDT", OpenTime: ingestionAt(7, 0, 0)}},
			expectedStartTime: ingestionAt(7, 5, 0),
		},
		{
			name:              "a gap wider than the lookback starts at the lookback",
			stored:            []entities.KCandle{{Symbol: "BTCUSDT", OpenTime: time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)}},
			expectedStartTime: time.Date(2026, 8, 29, 9, 7, 0, 0, time.UTC),
		},
		{
			name:              "a symbol that never held a candle fills the whole lookback",
			stored:            []entities.KCandle{},
			expectedStartTime: time.Date(2026, 8, 29, 9, 7, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
			underTest.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 1).Return(testCase.stored, nil)
			underTest.marketDataProxy.EXPECT().FetchKCandles(vo.NewKCandleFetchWindowVo(
				"BTCUSDT", testCase.expectedStartTime, ingestionAt(9, 0, 0))).
				Return([]vo.MarketKCandleVo{}, nil)

			_, runError := underTest.service.RunBackfill([]string{"BTCUSDT"})

			require.NoError(t, runError)
		})
	}
}

func TestBackfillNeverCallsTheSourceWhenThereIsNoGap(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 1).
		Return([]entities.KCandle{{Symbol: "BTCUSDT", OpenTime: ingestionAt(9, 0, 0)}}, nil)

	report, runError := underTest.service.RunBackfill([]string{"BTCUSDT"})

	require.NoError(t, runError)
	symbolReport := reportFor(t, report, "BTCUSDT")
	assert.Equal(t, 0, symbolReport.StoredCount)
	assert.Empty(t, symbolReport.FetchFailureReason)
}

func TestBackfillKeepsSymbolsIndependent(t *testing.T) {
	testCases := []struct {
		name         string
		arrangeFirst func(underTest ingestionUnderTest)
	}{
		{
			name: "reading the stored candle fails",
			arrangeFirst: func(underTest ingestionUnderTest) {
				underTest.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 1).
					Return(nil, errors.New("storage refused the read"))
			},
		},
		{
			name: "the source will not answer",
			arrangeFirst: func(underTest ingestionUnderTest) {
				underTest.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 1).Return([]entities.KCandle{}, nil)
				underTest.marketDataProxy.EXPECT().
					FetchKCandles(gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "BTCUSDT" })).
					Return(nil, sourceUnreachable)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
			testCase.arrangeFirst(underTest)
			underTest.kCandleRepository.EXPECT().FindLatest("ETHUSDT", 1).Return([]entities.KCandle{}, nil)
			underTest.marketDataProxy.EXPECT().
				FetchKCandles(gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "ETHUSDT" })).
				Return([]vo.MarketKCandleVo{reportedFor("ETHUSDT", ingestionAt(9, 0, 0))}, nil)
			saved := underTest.acceptEverySave()

			report, runError := underTest.service.RunBackfill([]string{"BTCUSDT", "ETHUSDT"})

			require.NoError(t, runError)
			assert.Len(t, saved.all(), 1)
			assert.NotEmpty(t, reportFor(t, report, "BTCUSDT").FetchFailureReason)
			assert.Equal(t, 1, reportFor(t, report, "ETHUSDT").StoredCount)
		})
	}
}

func TestBothUseCasesRefuseToRunOnAnUnusableCandleCount(t *testing.T) {
	testCases := []struct {
		name string
		run  func(ingestionService *service.KCandleIngestionService) (dto.KCandleIngestionReportDto, error)
	}{
		{
			name: "a scheduled round",
			run: func(ingestionService *service.KCandleIngestionService) (dto.KCandleIngestionReportDto, error) {
				return ingestionService.RunScheduledRound([]string{"BTCUSDT"})
			},
		},
		{
			name: "the backfill",
			run: func(ingestionService *service.KCandleIngestionService) (dto.KCandleIngestionReportDto, error) {
				return ingestionService.RunBackfill([]string{"BTCUSDT"})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockController := gomock.NewController(t)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 0)).AnyTimes()
			ingestionService := service.NewKCandleIngestionService(
				mocks.NewMockIKCandleRepository(mockController),
				mocks.NewMockIMarketDataProxy(mockController),
				clockProxy, 0, lookback)

			report, runError := testCase.run(ingestionService)

			require.ErrorIs(t, runError, domains.ErrKCandleIngestionValidation)
			assert.Empty(t, report.SymbolReports)
		})
	}
}

func TestTheNextRoundRefillsWhatAFailedRoundMissed(t *testing.T) {
	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)

	// Time moves on by one candle between the two rounds, so the second round is not
	// simply asking for the same window again.
	gomock.InOrder(
		clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 0)),
		clockProxy.EXPECT().Now().Return(ingestionAt(9, 12, 0)),
	)
	marketDataProxy.EXPECT().
		FetchKCandles(vo.NewKCandleFetchWindowVo("BTCUSDT", ingestionAt(8, 40, 0), ingestionAt(9, 0, 0))).
		Return(nil, sourceUnreachable)
	marketDataProxy.EXPECT().
		FetchKCandles(vo.NewKCandleFetchWindowVo("BTCUSDT", ingestionAt(8, 45, 0), ingestionAt(9, 5, 0))).
		Return([]vo.MarketKCandleVo{
			validReportedKCandle(ingestionAt(8, 45, 0)),
			validReportedKCandle(ingestionAt(8, 50, 0)),
			validReportedKCandle(ingestionAt(8, 55, 0)),
			validReportedKCandle(ingestionAt(9, 0, 0)),
			validReportedKCandle(ingestionAt(9, 5, 0)),
		}, nil)

	underTest := ingestionUnderTest{
		service: service.NewKCandleIngestionService(
			kCandleRepository, marketDataProxy, clockProxy, roundCandleCount, lookback),
		kCandleRepository: kCandleRepository,
		marketDataProxy:   marketDataProxy,
	}
	saved := underTest.acceptEverySave()

	failedReport, failedError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})
	recoveredReport, recoveredError := underTest.service.RunScheduledRound([]string{"BTCUSDT"})

	require.NoError(t, failedError)
	require.NoError(t, recoveredError)
	assert.Equal(t, 0, reportFor(t, failedReport, "BTCUSDT").StoredCount)
	assert.Contains(t, saved.all(), ingestionAt(9, 0, 0))
	assert.Equal(t, 5, reportFor(t, recoveredReport, "BTCUSDT").StoredCount)
}

// TestEveryWatchedSymbolIsUnderwayAtOnce holds the difference between "independent"
// and "at the same time", which the other tests cannot tell apart.
//
// The source refuses to answer either symbol until both have arrived. Symbols run
// one after another would deadlock on the first, so the second never arrives and the
// test fails on its own deadline rather than hanging the suite.
func TestEveryWatchedSymbolIsUnderwayAtOnce(t *testing.T) {
	arrived := make(chan string, 2)
	release := make(chan struct{})

	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).
		DoAndReturn(func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			arrived <- window.Symbol
			select {
			case <-release:
				return []vo.MarketKCandleVo{}, nil
			case <-time.After(3 * time.Second):
				return nil, errors.New("waited alone for the other symbol")
			}
		}).Times(2)

	finished := make(chan dto.KCandleIngestionReportDto, 1)
	go func() {
		report, _ := underTest.service.RunScheduledRound([]string{"BTCUSDT", "ETHUSDT"})
		finished <- report
	}()

	firstArrival := waitForArrival(t, arrived)
	secondArrival := waitForArrival(t, arrived)
	close(release)

	assert.ElementsMatch(t, []string{"BTCUSDT", "ETHUSDT"}, []string{firstArrival, secondArrival})
	report := waitForRound(t, finished)
	assert.Empty(t, reportFor(t, report, "BTCUSDT").FetchFailureReason)
	assert.Empty(t, reportFor(t, report, "ETHUSDT").FetchFailureReason)
}

func waitForArrival(t *testing.T, arrived chan string) string {
	t.Helper()

	select {
	case symbol := <-arrived:
		return symbol
	case <-time.After(2 * time.Second):
		t.Fatal("only one trading symbol ever reached the market source, so they are not running at once")
		return ""
	}
}

func waitForRound(t *testing.T, finished chan dto.KCandleIngestionReportDto) dto.KCandleIngestionReportDto {
	t.Helper()

	select {
	case report := <-finished:
		return report
	case <-time.After(2 * time.Second):
		t.Fatal("the round never finished")
		return dto.KCandleIngestionReportDto{}
	}
}
