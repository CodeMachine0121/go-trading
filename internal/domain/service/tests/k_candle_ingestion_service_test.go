package service_test

import (
	"context"
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

// reportedKCandle is a source-reported candle; high and low are explicit because the K candle rules judge that pair.
func reportedKCandle(openTime time.Time, high string, low string) vo.MarketKCandleVo {
	return vo.MarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString(high),
		Low:                 decimal.RequireFromString(low),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
	}
}

func validReportedKCandle(openTime time.Time) vo.MarketKCandleVo {
	return reportedKCandle(openTime, "120", "90")
}

func reportedFor(symbol string, openTime time.Time) vo.MarketKCandleVo {
	marketKCandle := validReportedKCandle(openTime)
	marketKCandle.Symbol = symbol

	return marketKCandle
}

// movingClock lets a test move the service into another day, to check a presumed closure is judged afresh.
type movingClock struct {
	mutex       sync.Mutex
	currentTime time.Time
}

func (clock *movingClock) now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()

	return clock.currentTime
}

func (clock *movingClock) moveTo(currentTime time.Time) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.currentTime = currentTime
}

type ingestionUnderTest struct {
	clock                    *movingClock
	service                  *service.KCandleIngestionService
	kCandleRepository        *mocks.MockIKCandleRepository
	historySyncRunRepository *mocks.MockIKCandleHistorySyncRunRepository
	tradingSymbolRepository  *mocks.MockITradingSymbolRepository
	marketDataProxy          *mocks.MockIMarketDataProxy
}

func newIngestionUnderTest(t *testing.T, currentTime time.Time) ingestionUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	historySyncRunRepository := mocks.NewMockIKCandleHistorySyncRunRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	movingClock := &movingClock{currentTime: currentTime}
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().DoAndReturn(movingClock.now).AnyTimes()
	// Sleep is stubbed so back-off never slows the tests.
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()

	return ingestionUnderTest{
		clock: movingClock, service: service.NewKCandleIngestionService(
			kCandleRepository, historySyncRunRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
			ingestionMarketCatalog(), roundCandleCount, lookback),
		kCandleRepository:        kCandleRepository,
		historySyncRunRepository: historySyncRunRepository,
		tradingSymbolRepository:  tradingSymbolRepository,
		marketDataProxy:          marketDataProxy,
	}
}

// ingestionMarketCatalog holds the round-the-clock market and a Taiwan session closing at 13:30.
func ingestionMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   time.FixedZone("Asia/Taipei", 8*60*60),
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			FollowsFixedRoster:         true,
			SimultaneousChannelCeiling: 1,
			SymbolsPerLiveChannel:      5,
		},
	})
}

// watching sets the watchlist the run reads; symbols default to the round-the-clock market.
func (underTest ingestionUnderTest) watching(symbols ...string) {
	underTest.watchingInMarket(vo.MarketCrypto, symbols...)
}

func (underTest ingestionUnderTest) watchingInMarket(market vo.MarketVo, symbols ...string) {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(market), IsWatched: true,
		})
	}

	underTest.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(watchedSymbols, nil).AnyTimes()
}

// syncingFromEmptyStorage makes every chunk empty and writes every absent batch whole.
// It is separate from acceptEverySave because several cases are about which path, replacing or insert-if-absent, a run reaches.
func (underTest ingestionUnderTest) syncingFromEmptyStorage() *savedOpenTimes {
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()

	saved := &savedOpenTimes{openTimes: []time.Time{}}
	underTest.kCandleRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, kCandles []entities.KCandle) (int, error) {
			for _, kCandle := range kCandles {
				saved.record(kCandle)
			}

			return len(kCandles), nil
		}).AnyTimes()

	return saved
}

// savedOpenTimes records what reached storage, safe across the round's per-symbol goroutines.
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
	underTest.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, kCandle entities.KCandle) (entities.KCandle, error) {
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
			currentTime:       ingestionAt(9, 7, 20),
			reported:          []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 6, 0))},
			expectedOpenTimes: []time.Time{ingestionAt(9, 6, 0)},
		},
		{
			name:        "the candle still running is left out",
			currentTime: ingestionAt(9, 7, 20),
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(9, 6, 0)),
				validReportedKCandle(ingestionAt(9, 7, 0)),
			},
			expectedOpenTimes: []time.Time{ingestionAt(9, 6, 0)},
		},
		{
			name:              "that same candle is stored once its interval has finished",
			currentTime:       ingestionAt(9, 8, 20),
			reported:          []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 7, 0))},
			expectedOpenTimes: []time.Time{ingestionAt(9, 7, 0)},
		},
		{
			name:        "every candle of a full round is stored",
			currentTime: ingestionAt(9, 7, 20),
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(9, 2, 0)),
				validReportedKCandle(ingestionAt(9, 3, 0)),
				validReportedKCandle(ingestionAt(9, 4, 0)),
				validReportedKCandle(ingestionAt(9, 5, 0)),
				validReportedKCandle(ingestionAt(9, 6, 0)),
			},
			expectedOpenTimes: []time.Time{
				ingestionAt(9, 2, 0), ingestionAt(9, 3, 0), ingestionAt(9, 4, 0),
				ingestionAt(9, 5, 0), ingestionAt(9, 6, 0),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, testCase.currentTime)
			underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(testCase.reported, nil)
			saved := underTest.acceptEverySave()

			underTest.watching("BTCUSDT")
			report, runError := underTest.service.RunScheduledRound(t.Context())

			require.NoError(t, runError)
			assert.Equal(t, testCase.expectedOpenTimes, saved.all())
			assert.Equal(t, len(testCase.expectedOpenTimes), reportFor(t, report, "BTCUSDT").StoredCount)
		})
	}
}

func TestScheduledRoundAsksForTheNewestClosedCandlesBackwards(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 20))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, ingestionAt(9, 2, 0), ingestionAt(9, 6, 0))).
		Return([]vo.MarketKCandleVo{}, nil)

	underTest.watching("BTCUSDT")
	_, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
}

func TestScheduledRoundCoversEveryWatchedSymbol(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			return []vo.MarketKCandleVo{reportedFor(window.Symbol, ingestionAt(9, 0, 0))}, nil
		}).Times(2)
	saved := underTest.acceptEverySave()

	underTest.watching("BTCUSDT", "ETHUSDT")
	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Len(t, saved.all(), 2)
	assert.Equal(t, 1, reportFor(t, report, "BTCUSDT").StoredCount)
	assert.Equal(t, 1, reportFor(t, report, "ETHUSDT").StoredCount)
}

func TestScheduledRoundOnAnEmptyWatchlistDoesNothing(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))

	underTest.watching()
	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Empty(t, report.SymbolReports)
}

func TestOneSymbolFailingLeavesTheOthersAlone(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "BTCUSDT" })).
		Return(nil, sourceUnreachable)
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "ETHUSDT" })).
		Return([]vo.MarketKCandleVo{reportedFor("ETHUSDT", ingestionAt(9, 0, 0))}, nil)
	saved := underTest.acceptEverySave()

	underTest.watching("BTCUSDT", "ETHUSDT")
	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Len(t, saved.all(), 1)
	assert.Equal(t, 0, reportFor(t, report, "BTCUSDT").StoredCount)
	assert.Contains(t, reportFor(t, report, "BTCUSDT").FetchFailureReason, "market source unreachable")
	assert.Equal(t, 1, reportFor(t, report, "ETHUSDT").StoredCount)
	assert.Empty(t, reportFor(t, report, "ETHUSDT").FetchFailureReason)
}

func TestEverySymbolFailingStillReportsRatherThanErroring(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable).Times(2)

	underTest.watching("BTCUSDT", "ETHUSDT")
	report, runError := underTest.service.RunScheduledRound(t.Context())

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
			name: "an open time off the one minute mark",
			reported: []vo.MarketKCandleVo{
				validReportedKCandle(ingestionAt(8, 50, 0)),
				validReportedKCandle(ingestionAt(8, 53, 30)),
				validReportedKCandle(ingestionAt(9, 0, 0)),
			},
			expectedStored: 2,
			expectedSkipped: []dto.SkippedKCandleDto{
				{OpenTime: ingestionAt(8, 53, 30), Reason: "起始時間必須落在1分鐘刻度上"},
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
			underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return(testCase.reported, nil)
			underTest.acceptEverySave()

			underTest.watching("BTCUSDT")
			report, runError := underTest.service.RunScheduledRound(t.Context())

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
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return([]vo.MarketKCandleVo{
		validReportedKCandle(ingestionAt(8, 55, 0)),
		validReportedKCandle(ingestionAt(9, 0, 0)),
	}, nil)
	underTest.kCandleRepository.EXPECT().
		Save(gomock.Any(), gomock.Cond(func(kCandle entities.KCandle) bool { return kCandle.OpenTime.Equal(ingestionAt(8, 55, 0)) })).
		Return(entities.KCandle{}, errors.New("storage refused the write"))
	underTest.kCandleRepository.EXPECT().
		Save(gomock.Any(), gomock.Cond(func(kCandle entities.KCandle) bool { return kCandle.OpenTime.Equal(ingestionAt(9, 0, 0)) })).
		Return(entities.KCandle{}, nil)

	underTest.watching("BTCUSDT")
	report, runError := underTest.service.RunScheduledRound(t.Context())

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
			expectedStartTime: ingestionAt(7, 1, 0),
		},
		{
			// The lookback lands mid-day, so the fetch starts at that day's edge rather than yield a half bucket.
			name:              "a gap wider than the lookback starts at the edge of the day it reaches",
			stored:            []entities.KCandle{{Symbol: "BTCUSDT", OpenTime: time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)}},
			expectedStartTime: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "a symbol that never held a candle starts at the edge of the day the lookback reaches",
			stored:            []entities.KCandle{},
			expectedStartTime: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
			underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).Return(testCase.stored, nil)
			underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, testCase.expectedStartTime, ingestionAt(9, 6, 0))).
				Return([]vo.MarketKCandleVo{}, nil)

			underTest.watching("BTCUSDT")
			_, runError := underTest.service.RunBackfill(t.Context())

			require.NoError(t, runError)
		})
	}
}

func TestBackfillNeverCallsTheSourceWhenThereIsNoGap(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{{Symbol: "BTCUSDT", OpenTime: ingestionAt(9, 6, 0)}}, nil)

	underTest.watching("BTCUSDT")
	report, runError := underTest.service.RunBackfill(t.Context())

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
				underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
					Return(nil, errors.New("storage refused the read"))
			},
		},
		{
			name: "the source will not answer",
			arrangeFirst: func(underTest ingestionUnderTest) {
				underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).Return([]entities.KCandle{}, nil)
				underTest.marketDataProxy.EXPECT().
					FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "BTCUSDT" })).
					Return(nil, sourceUnreachable)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
			testCase.arrangeFirst(underTest)
			underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "ETHUSDT", 1).Return([]entities.KCandle{}, nil)
			underTest.marketDataProxy.EXPECT().
				FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool { return window.Symbol == "ETHUSDT" })).
				Return([]vo.MarketKCandleVo{reportedFor("ETHUSDT", ingestionAt(9, 0, 0))}, nil)
			saved := underTest.acceptEverySave()

			underTest.watching("BTCUSDT", "ETHUSDT")
			report, runError := underTest.service.RunBackfill(t.Context())

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
				return ingestionService.RunScheduledRound(t.Context())
			},
		},
		{
			name: "the backfill",
			run: func(ingestionService *service.KCandleIngestionService) (dto.KCandleIngestionReportDto, error) {
				return ingestionService.RunBackfill(t.Context())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockController := gomock.NewController(t)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 0)).AnyTimes()
			// No watchlist expectation is set, so an unworkable run reaching storage fails the test.
			ingestionService := service.NewKCandleIngestionService(
				mocks.NewMockIKCandleRepository(mockController),
				mocks.NewMockIKCandleHistorySyncRunRepository(mockController),
				mocks.NewMockITradingSymbolRepository(mockController),
				mocks.NewMockIMarketDataProxy(mockController),
				clockProxy, ingestionMarketCatalog(), 0, lookback)

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

	// Time advances one candle between rounds, so the second asks for a new window.
	gomock.InOrder(
		clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 20)),
		clockProxy.EXPECT().Now().Return(ingestionAt(9, 8, 20)),
	)
	marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, ingestionAt(9, 2, 0), ingestionAt(9, 6, 0))).
		Return(nil, sourceUnreachable)
	marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, ingestionAt(9, 3, 0), ingestionAt(9, 7, 0))).
		Return([]vo.MarketKCandleVo{
			validReportedKCandle(ingestionAt(9, 3, 0)),
			validReportedKCandle(ingestionAt(9, 4, 0)),
			validReportedKCandle(ingestionAt(9, 5, 0)),
			validReportedKCandle(ingestionAt(9, 6, 0)),
			validReportedKCandle(ingestionAt(9, 7, 0)),
		}, nil)

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	historySyncRunRepository := mocks.NewMockIKCandleHistorySyncRunRepository(mockController)
	underTest := ingestionUnderTest{service: service.NewKCandleIngestionService(
		kCandleRepository, historySyncRunRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
		ingestionMarketCatalog(), roundCandleCount, lookback),
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		marketDataProxy:         marketDataProxy,
	}
	saved := underTest.acceptEverySave()
	underTest.watching("BTCUSDT")

	failedReport, failedError := underTest.service.RunScheduledRound(t.Context())
	recoveredReport, recoveredError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, failedError)
	require.NoError(t, recoveredError)
	assert.Equal(t, 0, reportFor(t, failedReport, "BTCUSDT").StoredCount)
	assert.Contains(t, saved.all(), ingestionAt(9, 6, 0))
	assert.Equal(t, 5, reportFor(t, recoveredReport, "BTCUSDT").StoredCount)
}

// TestEveryWatchedSymbolIsUnderwayAtOnce proves concurrency: the source answers neither symbol until both arrive, so sequential processing deadlocks and fails on the deadline.
func TestEveryWatchedSymbolIsUnderwayAtOnce(t *testing.T) {
	arrived := make(chan string, 2)
	release := make(chan struct{})

	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
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
		underTest.watching("BTCUSDT", "ETHUSDT")
		report, _ := underTest.service.RunScheduledRound(t.Context())
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

// Passing an already-done context and seeing it at the source proves the round does not make its own context.
func TestTheContextARoundIsGivenIsTheOneItsSourceIsCalledUnder(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	callerWentAway, abandonTheRound := context.WithCancel(t.Context())
	abandonTheRound()

	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			executionContext context.Context, _ vo.KCandleFetchWindowVo,
		) ([]vo.MarketKCandleVo, error) {
			assert.ErrorIs(t, executionContext.Err(), context.Canceled)
			return []vo.MarketKCandleVo{}, nil
		})

	underTest.watching("BTCUSDT")
	_, err := underTest.service.RunScheduledRound(callerWentAway)

	assert.NoError(t, err)
}

// taipeiIngestionAt parses a moment in Taipei time, the Taiwan session's clock.
func taipeiIngestionAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

func TestARoundSkipsAMarketThatCouldHoldNothing(t *testing.T) {
	testCases := []struct {
		name           string
		currentTime    string
		expectedAsking bool
	}{
		{
			name:        "mid session, so it is asked",
			currentTime: "2026-09-08T10:07:00+08:00", expectedAsking: true,
		},
		{
			// The last candle finished at 13:30, so a round three minutes later still collects it.
			name:        "just after the close, the day's last candle is still collected",
			currentTime: "2026-09-08T13:33:00+08:00", expectedAsking: true,
		},
		{
			name:        "the evening",
			currentTime: "2026-09-08T21:00:00+08:00", expectedAsking: false,
		},
		{
			name:        "sunday",
			currentTime: "2026-09-13T10:07:00+08:00", expectedAsking: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, testCase.currentTime))
			underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
			if testCase.expectedAsking {
				underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
					Return([]vo.MarketKCandleVo{}, nil)
			}

			report, runError := underTest.service.RunScheduledRound(t.Context())

			require.NoError(t, runError)
			// A shut market is not a failure; reporting it every round would bury real ones.
			assert.Empty(t, reportFor(t, report, "2330").FetchFailureReason)
		})
	}
}

func TestAClosedMarketDoesNotStopAnotherOneBeingFetched(t *testing.T) {
	// The round-the-clock market must not be held back by the closed Taiwan evening.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T21:00:00+08:00"))
	underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.TradingSymbol{
			{Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true},
			{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
		}, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool {
			return window.Symbol == "BTCUSDT"
		})).Return([]vo.MarketKCandleVo{}, nil)

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	assert.Empty(t, reportFor(t, report, "2330").FetchFailureReason)
	assert.Empty(t, reportFor(t, report, "BTCUSDT").FetchFailureReason)
}

func TestAMarketThatAnswersWithNothingIsPresumedShutForItsOwnDay(t *testing.T) {
	// A holiday looks like this: the source answers fine with no candles for the whole market, so no calendar needs maintaining.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T10:07:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	// Asked exactly once; later rounds must not reach the source again.
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).Times(1)

	_, firstError := underTest.service.RunScheduledRound(t.Context())
	_, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
}

func TestAMarketPresumedShutIsAskedAgainTheFollowingDay(t *testing.T) {
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T10:07:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).Times(1)
	_, firstError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, firstError)

	// The next day is judged afresh.
	underTest.clock.moveTo(taipeiIngestionAt(t, "2026-09-09T10:07:00+08:00"))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).Times(1)

	_, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, secondError)
}

func TestASourceThatWillNotAnswerIsNeverReadAsAHoliday(t *testing.T) {
	// Only an empty answer counts as a holiday; an unreachable source must be reported, not quietly left unfetched.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T10:07:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable).Times(2)

	firstReport, firstError := underTest.service.RunScheduledRound(t.Context())
	secondReport, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.NotEmpty(t, reportFor(t, firstReport, "2330").FetchFailureReason)
	assert.NotEmpty(t, reportFor(t, secondReport, "2330").FetchFailureReason)
}

func TestOneQuietSymbolDoesNotShutTheWholeMarket(t *testing.T) {
	// One untraded stock is not a holiday; every symbol of the market must come back empty.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T10:07:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330", "2454")
	underTest.acceptEverySave()
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool {
			return window.Symbol == "2330"
		})).Return([]vo.MarketKCandleVo{}, nil).Times(2)
	underTest.marketDataProxy.EXPECT().
		FetchKCandles(gomock.Any(), gomock.Cond(func(window vo.KCandleFetchWindowVo) bool {
			return window.Symbol == "2454"
		})).Return([]vo.MarketKCandleVo{
		reportedFor("2454", taipeiIngestionAt(t, "2026-09-08T10:00:00+08:00")),
	}, nil).Times(2)

	_, firstError := underTest.service.RunScheduledRound(t.Context())
	_, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
}

func TestARoundReadsTheWatchlistAfresh(t *testing.T) {
	// A symbol added while running is fetched by the very next round.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.acceptEverySave()
	gomock.InOrder(
		underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			[]entities.TradingSymbol{
				{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
			}, nil),
		underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			[]entities.TradingSymbol{
				{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
				{Symbol: "ETHUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
			}, nil),
	)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	firstReport, firstError := underTest.service.RunScheduledRound(t.Context())
	secondReport, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.Len(t, firstReport.SymbolReports, 1)
	assert.Len(t, secondReport.SymbolReports, 2)
}

func TestARunReportsAFailureReadingTheWatchlist(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	storageFailure := errors.New("storage unreachable")
	underTest.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(nil, storageFailure)

	_, runError := underTest.service.RunScheduledRound(t.Context())

	assert.ErrorIs(t, runError, storageFailure)
}

func TestBackfillOnlyReachesBackIntoTradingSessions(t *testing.T) {
	// Starting on a Saturday, only Friday's session within reach is asked for.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-12T09:00:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock,
		taipeiIngestionAt(t, "2026-09-11T09:00:00+08:00"),
		taipeiIngestionAt(t, "2026-09-11T13:29:00+08:00"),
	)).Return([]vo.MarketKCandleVo{}, nil)

	_, runError := underTest.service.RunBackfill(t.Context())

	require.NoError(t, runError)
}

func TestBackfillAsksForNothingWhenNothingInReachCouldTrade(t *testing.T) {
	// Sunday morning with a weekend-only lookback: the market was shut, so there is no gap to ask for or report.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-13T09:00:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)

	report, runError := underTest.service.RunBackfill(t.Context())

	require.NoError(t, runError)
	assert.Empty(t, reportFor(t, report, "2330").FetchFailureReason)
	assert.Equal(t, 0, reportFor(t, report, "2330").StoredCount)
}

func TestCatchingOneSymbolUpAsksForItsOwnGapAfterTheCloseHasPassed(t *testing.T) {
	// On a trading-day evening the scheduled round has nothing left, so the on-demand catch-up must still reach the session.
	// Reaching back a day from 20:00 lands mid-day and starts at that day's edge, so the previous session is included rather than left as a hole.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))
	underTest.acceptEverySave()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock,
		taipeiIngestionAt(t, "2026-09-10T09:00:00+08:00"),
		taipeiIngestionAt(t, "2026-09-11T13:29:00+08:00"),
	)).Return([]vo.MarketKCandleVo{}, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "2330")

	require.NoError(t, catchUpError)
}

func TestCatchingOneSymbolUpReachesASymbolNobodyIsWatching(t *testing.T) {
	// A chart can be opened for an unwatched symbol, so the catch-up must reach it by name rather than via the watchlist.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))
	underTest.acceptEverySave()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: false,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "2330")

	require.NoError(t, catchUpError)
}

func TestCatchingUpASymbolNobodyRegisteredIsRefused(t *testing.T) {
	// The market comes only from registration; it is deliberately never guessed from the name.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "9999").
		Return(entities.TradingSymbol{}, false, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "9999")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNotRegistered)
}

func TestCatchingUpNothingIsRefusedAsAName(t *testing.T) {
	// Blank is not an unregistered symbol but no symbol at all, so the caller should retype rather than register.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "   ")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNamed)
	assert.NotErrorIs(t, catchUpError, domains.ErrTradingSymbolNotRegistered)
}

func TestCatchingOneSymbolUpNeverDecidesItsWholeMarketIsShut(t *testing.T) {
	// One symbol's silence must not latch its whole market shut from a single button press.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T10:00:00+08:00"))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "2330")
	require.NoError(t, catchUpError)

	// The following scheduled round still asks the source, which it would not for a market presumed shut.
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2454")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)

	report, roundError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, roundError)
	assert.True(t, reportFor(t, report, "2454").WasAsked)
}

func TestCatchingOneSymbolUpAsksEvenWhenItsMarketWasDecidedShut(t *testing.T) {
	// A presumed closure is an inference (a late source looks like a holiday), so a manual request must clear it rather than report nothing collected.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T12:00:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)
	_, roundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, roundError)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
		Return([]entities.KCandle{}, nil)
	askedAgain := make(chan struct{}, 1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			askedAgain <- struct{}{}

			return []vo.MarketKCandleVo{}, nil
		})

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "2330")

	require.NoError(t, catchUpError)
	assert.Len(t, askedAgain, 1, "手動補齊要真的去問，不能被今日推定休市擋下來")
}

func TestAMarketIsNotDecidedShutBeforeItsSilenceMeansAnything(t *testing.T) {
	// Three minutes after the bell a late source empties every symbol, so silence only latches a holiday once the session has run as long as the round covers.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T09:03:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)
	_, roundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, roundError)

	// Later the source has caught up; a market latched shut at 09:03 would never be asked again today.
	underTest.clock.moveTo(taipeiIngestionAt(t, "2026-09-08T09:12:00+08:00"))
	askedAgain := make(chan struct{}, 1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			askedAgain <- struct{}{}

			return []vo.MarketKCandleVo{}, nil
		})

	_, laterRoundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, laterRoundError)

	assert.Len(t, askedAgain, 1, "開盤沒多久的一次空手，不足以判定整天休市")
}

func TestARoundInFlightWorksFromTheListItStartedWith(t *testing.T) {
	// The watchlist is read once per round, so a mid-round change waits for the next round and its fresh "now".
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 0))
	underTest.acceptEverySave()
	changedMidRound := make(chan struct{})
	underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.TradingSymbol, error) {
			return []entities.TradingSymbol{
				{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
			}, nil
		}).Times(1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			// The watchlist changes mid-round; no second read is expected, so looking again fails the test.
			close(changedMidRound)

			return []vo.MarketKCandleVo{}, nil
		})

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	<-changedMidRound
	assert.Len(t, report.SymbolReports, 1)
}

const historyCeilingDays = 90

func historySyncOf(symbol string, lookbackDays int) dto.KCandleHistorySyncDto {
	return dto.KCandleHistorySyncDto{Symbol: symbol, LookbackDays: lookbackDays}
}

// historySyncRuns collects the run rows a sync writes, since the fetch outlives the request and a case can only watch the run until it stops running.
type historySyncRuns struct {
	mutex      sync.Mutex
	written    []entities.KCandleHistorySyncRun
	ended      chan entities.KCandleHistorySyncRun
	endedOnce  sync.Once
	nextRunID  uint
	saveFailed error
}

func (runs *historySyncRuns) record(
	syncRun entities.KCandleHistorySyncRun,
) (entities.KCandleHistorySyncRun, error) {
	runs.mutex.Lock()
	if syncRun.ID == 0 {
		runs.nextRunID++
		syncRun.ID = runs.nextRunID
	}
	runs.written = append(runs.written, syncRun)
	runs.mutex.Unlock()

	if vo.NewKCandleHistorySyncRunStatusVo(syncRun.Status) != vo.KCandleHistorySyncRunning {
		runs.endedOnce.Do(func() { runs.ended <- syncRun })
	}

	return syncRun, runs.saveFailed
}

// awaitEnding returns the final run write, bounded so a sync that never ends fails instead of hanging.
func (runs *historySyncRuns) awaitEnding(t *testing.T) entities.KCandleHistorySyncRun {
	t.Helper()

	select {
	case endedRun := <-runs.ended:
		return endedRun
	case <-time.After(5 * time.Second):
		t.Fatal("歷史同步沒有收尾")

		return entities.KCandleHistorySyncRun{}
	}
}

// progressFigures returns each write's completed-chunk count, in order.
func (runs *historySyncRuns) progressFigures() []int {
	runs.mutex.Lock()
	defer runs.mutex.Unlock()

	figures := make([]int, 0, len(runs.written))
	for _, syncRun := range runs.written {
		figures = append(figures, syncRun.CompletedChunks)
	}

	return figures
}

// recordsEveryHistorySyncRun keeps every written version of the run so a case can read progress and ending together.
func (underTest ingestionUnderTest) recordsEveryHistorySyncRun() *historySyncRuns {
	runs := &historySyncRuns{ended: make(chan entities.KCandleHistorySyncRun, 1)}
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleHistorySyncRun,
		) (entities.KCandleHistorySyncRun, error) {
			return runs.record(syncRun)
		}).AnyTimes()

	return runs
}

func TestSyncingHistoryAsksForTheWholeStretchTheCallerNamed(t *testing.T) {
	// The start comes only from the lookback, unlike a backfill, which starts after the stored data.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	askedWindows := underTest.recordEveryWindowAskedAbout(
		[]vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)

	// The chunks must together span exactly the requested stretch with no gap.
	require.NotEmpty(t, *askedWindows)
	// Two days back from 2026-08-30 09:07, rounded down to a bucket edge.
	assert.Equal(t, time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), (*askedWindows)[0].StartTime)
	assert.Equal(t, ingestionAt(9, 6, 0), (*askedWindows)[len(*askedWindows)-1].EndTime)
	for index := 1; index < len(*askedWindows); index++ {
		assert.Equal(t,
			(*askedWindows)[index-1].EndTime.Add(time.Minute),
			(*askedWindows)[index].StartTime,
			"一段接一段之間不可以漏掉任何一分鐘")
	}
	assert.Equal(t, len(*askedWindows), endedRun.StoredCount)
}

// recordEveryWindowAskedAbout answers every chunk with the same candles and records the windows in order.
func (underTest ingestionUnderTest) recordEveryWindowAskedAbout(
	answer []vo.MarketKCandleVo,
) *[]vo.KCandleFetchWindowVo {
	askedWindows := &[]vo.KCandleFetchWindowVo{}
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			*askedWindows = append(*askedWindows, window)

			return answer, nil
		}).AnyTimes()

	return askedWindows
}

func TestSyncingHistoryAnswersBeforeItHasFetchedAnything(t *testing.T) {
	// A multi-year fetch outlives any connection, so the run is recorded and returned before fetching.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	letGo := make(chan struct{})
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			<-letGo

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	startedRun, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	// It returned while the source is still blocked.
	require.NoError(t, startError)
	assert.NotZero(t, startedRun.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncRunning), startedRun.Status)
	assert.Positive(t, startedRun.TotalChunks, "回覆裡就要說出這一趟總共有幾段")

	close(letGo)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.NotNil(t, endedRun.FinishedAt, "收尾的輪次要說得出它什麼時候收的")
}

func TestSyncingHistoryMovesItsProgressAlongAsItGoes(t *testing.T) {
	// Progress only at the end would look like a stalled run.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)

	assert.Equal(t, endedRun.TotalChunks, endedRun.CompletedChunks)
	assert.Equal(t, []int{0, 0, 1, 2, 3, 3}, runs.progressFigures(),
		"進度要一段一段往前，不是只在收尾時才出現")
}

func TestSyncingHistoryAsksAboutTheWholeStretchEvenWhereItAlreadyHasData(t *testing.T) {
	// A backfill never revisits a hole in the middle, so the whole stretch is asked about without narrowing by stored data.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	askedWindows := underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	runs.awaitEnding(t)
	require.NotEmpty(t, *askedWindows)
	assert.Equal(t, time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), (*askedWindows)[0].StartTime)
}

func TestSyncingHistorySkipsAStretchItAlreadyHoldsEveryCandleOf(t *testing.T) {
	// Complete chunks are skipped after a cheap count; stored data decides whether to ask, never where to start, so middle holes stay reachable.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	// Only the first day back is complete; one candle short is not, which the next case pins.
	completeDay := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, startTime time.Time, _ time.Time) (int, error) {
			if startTime.Equal(completeDay) {
				return 24 * 60, nil
			}

			return 0, nil
		}).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	askedWindows := underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	runs.awaitEnding(t)
	require.NotEmpty(t, *askedWindows)
	for _, askedWindow := range *askedWindows {
		assert.False(t, askedWindow.StartTime.Equal(completeDay),
			"已經齊全的那一天不應該再去問來源")
	}
}

func TestSyncingHistoryStillAsksAboutAStretchItIsOneCandleShortOf(t *testing.T) {
	// The skip is safe only while "complete" means every minute; one short is a hole that must be asked about.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	wholeDay := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, startTime time.Time, _ time.Time) (int, error) {
			if startTime.Equal(wholeDay) {
				return 24*60 - 1, nil
			}

			return 0, nil
		}).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	askedWindows := underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	runs.awaitEnding(t)

	askedAboutThatDay := false
	for _, askedWindow := range *askedWindows {
		if askedWindow.StartTime.Equal(wholeDay) {
			askedAboutThatDay = true
		}
	}
	assert.True(t, askedAboutThatDay, "差一根就不算齊全，那一段還是要問")
}

func TestSyncingHistorySkipsACompleteStretchOnAMarketThatCloses(t *testing.T) {
	// On a sessioned market "complete" means the session's minutes, not the whole day, or it would never skip or would skip a morning-only day.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T14:00:00+08:00"))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)

	// Every session asked about is answered as held in full.
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, startTime time.Time, endTime time.Time,
		) (int, error) {
			return int(endTime.Sub(startTime)/time.Minute) + 1, nil
		}).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("2330", 2), historyCeilingDays)

	require.NoError(t, startError)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), runs.awaitEnding(t).Status)
}

func TestSyncingHistoryKeepsWhatItAlreadyStoredWhenTheSourceGivesUpPartWay(t *testing.T) {
	// Storing per chunk means a source failing halfway leaves the first half stored, and the rest is abandoned since the source would keep refusing.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()

	storedBatches := 0
	underTest.kCandleRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, kCandles []entities.KCandle) (int, error) {
			storedBatches++

			return len(kCandles), nil
		}).AnyTimes()

	askedTimes := 0
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			askedTimes++
			if askedTimes > 1 {
				return nil, sourceUnreachable
			}

			return []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))}, nil
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)

	// The first chunk is stored before the second is asked for.
	assert.Equal(t, 1, storedBatches)
	assert.Equal(t, 1, endedRun.StoredCount)
	assert.Contains(t, endedRun.FetchFailureReason, "unreachable")
	// It stopped at the refusal instead of walking the rest of the stretch.
	assert.Equal(t, 2, askedTimes)
}

func TestSyncingHistoryCountsEverySkippedKCandle(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	brokenKCandles := make([]vo.MarketKCandleVo, 0, 300)
	for minute := range 300 {
		brokenKCandles = append(brokenKCandles,
			reportedKCandle(ingestionAt(0, minute, 0), "90", "120"))
	}
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(brokenKCandles, nil).Times(1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	assert.Equal(t, 300, runs.awaitEnding(t).SkippedCount)
}

func TestAnAbortedHistorySyncReportsTheChunkItReachedRatherThanTheWholeStretch(t *testing.T) {
	// A run that stopped early must not report the planned total, or it reads as finished.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(7, nil).AnyTimes()

	askedTimes := 0
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			askedTimes++
			if askedTimes > 1 {
				return nil, sourceUnreachable
			}

			return []vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))}, nil
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)

	assert.Equal(t, 3, endedRun.TotalChunks)
	assert.Equal(t, 1, endedRun.CompletedChunks, "只走完一段就停了，不能說三段都走完")
	// The first chunk's candles are still stored, so the run must report them.
	assert.Equal(t, 7, endedRun.StoredCount)
}

func TestAHistorySyncSaysWhenItFinished(t *testing.T) {
	// The ending time is read at the end, or every run would look instantaneous.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	// An hour passes between acceptance and finish.
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			underTest.clock.moveTo(ingestionAt(10, 7, 30))

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	startedRun, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)

	require.NotNil(t, endedRun.FinishedAt)
	assert.True(t, endedRun.FinishedAt.After(startedRun.StartedAt),
		"收尾時間要是收尾當下讀的，不是接下這一趟時讀的")
}

func TestAHistorySyncTriesAgainWhenTheClosingWriteWillNotLand(t *testing.T) {
	// A lost progress write costs one stale number, but a lost closing write leaves the row running until restart, so the closing write is retried.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	runs := &historySyncRuns{ended: make(chan entities.KCandleHistorySyncRun, 1)}
	closingAttempts := 0
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleHistorySyncRun,
		) (entities.KCandleHistorySyncRun, error) {
			if vo.NewKCandleHistorySyncRunStatusVo(syncRun.Status) == vo.KCandleHistorySyncRunning {
				return runs.record(syncRun)
			}

			// The first closing attempt fails.
			closingAttempts++
			if closingAttempts == 1 {
				return entities.KCandleHistorySyncRun{}, errors.New("storage unavailable")
			}

			return runs.record(syncRun)
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Equal(t, 2, closingAttempts, "收尾那一筆要再試一次，不能讓輪次一直掛在 running")
}

func TestSyncingHistoryEndsAsFailedWhenStorageBreaks(t *testing.T) {
	// A storage failure is the system's fault, so it ends the run rather than counting as a skipped candle, and is distinct from a source refusal.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	// The first chunk lands; storage breaks on the second.
	storedBatches := 0
	underTest.kCandleRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ []entities.KCandle) (int, error) {
			storedBatches++
			if storedBatches > 1 {
				return 0, errors.New("storage unavailable")
			}

			return 5, nil
		}).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))}, nil).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
	assert.Contains(t, endedRun.FailureReason, "storage unavailable")
	assert.Empty(t, endedRun.FetchFailureReason)
	// Reporting zero would send somebody back to refetch the whole stretch.
	assert.Equal(t, 5, endedRun.StoredCount)
	assert.Equal(t, 1, endedRun.CompletedChunks)
}

func TestSyncingHistoryRefusesToStartWhenTheRunCannotBeRecorded(t *testing.T) {
	// A run nobody can find cannot be observed or swept, which is worse than not starting.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleHistorySyncRun{}, errors.New("storage unavailable"))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.Error(t, startError)
}

func TestSyncingHistoryLeavesAlreadyStoredKCandlesAlone(t *testing.T) {
	// It fills only missing candles, unlike the scheduled round, which legitimately overwrites candles collected while forming.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{
			validReportedKCandle(ingestionAt(9, 4, 0)),
			validReportedKCandle(ingestionAt(9, 5, 0)),
		}, nil).Times(1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	// The store reports one of the two as already held, so only the other is new.
	underTest.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, kCandles []entities.KCandle) (int, error) {
			if len(kCandles) == 0 {
				return 0, nil
			}

			return 1, nil
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	// The count is only newly stored candles, so a complete stretch reports nothing stored.
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, 1, endedRun.StoredCount)
	assert.Equal(t, 0, endedRun.SkippedCount)
}

func TestTheOtherIngestionPathsStillOverwriteWhatTheyCollected(t *testing.T) {
	// The next round refetches a just-closed candle once the source settles it; refusing to overwrite would freeze the roughest version.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.watchingInMarket(vo.MarketCrypto, "BTCUSDT")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))}, nil)
	underTest.kCandleRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Times(0)
	saved := underTest.acceptEverySave()

	_, roundError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, roundError)
	assert.Equal(t, []time.Time{ingestionAt(9, 5, 0)}, saved.all())
}

func TestSyncingHistoryRefusesALookbackThatIsNotAStretch(t *testing.T) {
	testCases := []struct {
		name         string
		lookbackDays int
	}{
		{name: "no days at all", lookbackDays: 0},
		{name: "backwards", lookbackDays: -1},
		{name: "past the ceiling", lookbackDays: historyCeilingDays + 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Refused before any read, and without leaving a run behind.
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
			underTest.tradingSymbolRepository.EXPECT().
				FindBySymbol(gomock.Any(), gomock.Any()).Times(0)
			underTest.historySyncRunRepository.EXPECT().
				Save(gomock.Any(), gomock.Any()).Times(0)

			_, startError := underTest.service.StartHistorySyncFor(
				t.Context(), historySyncOf("BTCUSDT", testCase.lookbackDays), historyCeilingDays)

			assert.ErrorIs(t, startError, domains.ErrKCandleHistoryLookback)
		})
	}
}

func TestSyncingHistoryRefusesASymbolNobodyRegistered(t *testing.T) {
	// Same rule as the on-demand catch-up: no registration, no market, no source.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "9999").
		Return(entities.TradingSymbol{}, false, nil)
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("9999", 2), historyCeilingDays)

	assert.ErrorIs(t, startError, domains.ErrTradingSymbolNotRegistered)
}

func TestSyncingHistoryRefusesNothingAsAName(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("   ", 2), historyCeilingDays)

	assert.ErrorIs(t, startError, domains.ErrTradingSymbolNamed)
	assert.NotErrorIs(t, startError, domains.ErrTradingSymbolNotRegistered)
}

func TestSyncingHistoryReachesASymbolNobodyIsWatching(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: false,
		}, true, nil)
	underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), runs.awaitEnding(t).Status)
}

func TestSyncingHistoryReportsASourceThatWouldNotAnswer(t *testing.T) {
	// A source refusal is something the run found out, so the run still finishes carrying the reason.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Contains(t, endedRun.FetchFailureReason, "unreachable")
	assert.Empty(t, endedRun.FailureReason)
}

func TestSyncingHistoryDropsAStandingDecisionThatTheMarketIsShut(t *testing.T) {
	// A manual request must clear a presumed holiday rather than report nothing collected; this pins that the history sync path reaches the same code.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T10:00:00+08:00"))
	underTest.syncingFromEmptyStorage()
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.TradingSymbol{{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}}, nil).AnyTimes()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil).AnyTimes()

	// A scheduled round hearing nothing from every watched symbol latches the market shut.
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)
	_, roundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, roundError)

	// The next scheduled round obeys that and skips the source; the history sync right after must not.
	askedAgain := make(chan struct{}, 1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			select {
			case askedAgain <- struct{}{}:
			default:
			}

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("2330", 2), historyCeilingDays)

	require.NoError(t, startError)
	runs.awaitEnding(t)
	assert.Len(t, askedAgain, 1, "手動同步必須真的去問來源，而不是沿用「今天休市」那個判斷")
}

func TestSyncingHistoryOverAClosedMarketIsNotAFailure(t *testing.T) {
	// A stretch entirely outside the session holds no candle, which is normal and distinct from a source that would not answer.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-13T10:00:00+08:00"))
	runs := underTest.recordsEveryHistorySyncRun()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)
	// The window is all weekend, so the source is never reached.
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), historySyncOf("2330", 1), historyCeilingDays)

	require.NoError(t, startError)
	endedRun := runs.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Empty(t, endedRun.FetchFailureReason)
	assert.Equal(t, 0, endedRun.StoredCount)
}

func TestGettingAHistorySyncAnswersWithWhereItGotTo(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.historySyncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
		entities.KCandleHistorySyncRun{
			ID: 7, Symbol: "BTCUSDT", LookbackDays: 30,
			Status:      string(vo.KCandleHistorySyncRunning),
			TotalChunks: 30, CompletedChunks: 11, StoredCount: 15840,
			StartedAt: ingestionAt(9, 0, 0),
		}, true, nil)

	syncRun, findError := underTest.service.GetHistorySyncRun(t.Context(), 7)

	require.NoError(t, findError)
	assert.Equal(t, 11, syncRun.CompletedChunks)
	assert.Equal(t, 30, syncRun.TotalChunks)
	assert.Nil(t, syncRun.FinishedAt)
}

func TestGettingAHistorySyncNobodyStartedIsToldApartFromOneThatBroke(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.historySyncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
		entities.KCandleHistorySyncRun{}, false, nil)

	_, findError := underTest.service.GetHistorySyncRun(t.Context(), 7)

	assert.ErrorIs(t, findError, service.ErrKCandleHistorySyncRunNotFound)
}

func TestClearingInterruptedHistorySyncsSaysHowManyThereWere(t *testing.T) {
	// Runs live only in this process, so any still recorded as running has nothing fetching for it.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.historySyncRunRepository.EXPECT().
		FailAllRunning(gomock.Any(), gomock.Any(), gomock.Any()).Return(3, nil)

	clearedCount, sweepError := underTest.service.FailInterruptedHistorySyncs(t.Context())

	require.NoError(t, sweepError)
	assert.Equal(t, 3, clearedCount)
}
