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
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
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

// movingClock lets a test walk the same service into another day, which is the only
// way to check that a market presumed shut is judged afresh tomorrow.
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
	clock                   *movingClock
	service                 *service.KCandleIngestionService
	kCandleRepository       *mocks.MockIKCandleRepository
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	marketDataProxy         *mocks.MockIMarketDataProxy
}

func newIngestionUnderTest(t *testing.T, currentTime time.Time) ingestionUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	movingClock := &movingClock{currentTime: currentTime}
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().DoAndReturn(movingClock.now).AnyTimes()

	return ingestionUnderTest{
		clock: movingClock,
		service: service.NewKCandleIngestionService(
			kCandleRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
			ingestionMarketCatalog(), roundCandleCount, lookback),
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		marketDataProxy:         marketDataProxy,
	}
}

// ingestionMarketCatalog is the two markets these tests are written against: the
// round-the-clock one, and a Taiwan session that closes at half past one.
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
			SimultaneousChannelCeiling: 1,
			SymbolsPerLiveChannel:      5,
		},
	})
}

// watching is the watchlist this run reads at its top. The symbols belong to the
// round-the-clock market unless a test says otherwise, which is what keeps every
// existing rule about rounds and backfills readable without a market in sight.
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

// syncingFromEmptyStorage is a store holding nothing yet: every chunk counts as
// having no candles, and every batch of absent ones is written whole.
//
// It is separate from acceptEverySave because which of the two a run reaches is
// itself the thing several cases are about — one path replaces what it collected,
// the other must never touch what is already there.
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
			// Reaching back by the lookback lands mid-day, so the fetch begins at that
			// day's edge — otherwise the oldest bucket it produces is half a bucket.
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
			// No expectation is set on the watchlist, so reaching storage at all
			// would fail this test: a run that cannot work whatever it is pointed at
			// has no business reading a list it is about to throw away.
			ingestionService := service.NewKCandleIngestionService(
				mocks.NewMockIKCandleRepository(mockController),
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

	// Time moves on by one candle between the two rounds, so the second round is not
	// simply asking for the same window again.
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
	underTest := ingestionUnderTest{
		service: service.NewKCandleIngestionService(
			kCandleRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
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

// A round that reaches the outside world under a context of its own making would
// look identical from here until the day something needed cancelling. Handing in a
// context that is already done and finding it again at the far end is what tells the
// two apart.
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

// taipeiIngestionAt is a moment said in Taipei time, which is the clock the Taiwan
// trading session is written in.
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
			// The day's last candle finished at half past one; a round three minutes
			// later still has it to collect.
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
			// Either way this is not a failure: a market that is shut has nothing to
			// say, and saying so once every round would bury the rounds that matter.
			assert.Empty(t, reportFor(t, report, "2330").FetchFailureReason)
		})
	}
}

func TestAClosedMarketDoesNotStopAnotherOneBeingFetched(t *testing.T) {
	// The round-the-clock market trades through the Taiwan evening, and must not be
	// held back by a market that does not.
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
	// A holiday looks exactly like this from here: the source answers perfectly well
	// and has not one candle for anything in that market. Reading it off the answers
	// is what saves anybody maintaining a calendar that is wrong on the days it counts.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T10:07:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	// Asked exactly once. The rounds after the first must not reach the source again.
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

	// The next day is a fresh judgement — a holiday is one day off, not a verdict.
	underTest.clock.moveTo(taipeiIngestionAt(t, "2026-09-09T10:07:00+08:00"))
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).Times(1)

	_, secondError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, secondError)
}

func TestASourceThatWillNotAnswerIsNeverReadAsAHoliday(t *testing.T) {
	// Answered-with-nothing and did-not-answer are the only reliable distinction
	// available here, and conflating them would leave a broken source quietly
	// unfetched for the rest of the day instead of reported.
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
	// A single stock nobody traded for a minute is not a holiday. Requiring every
	// symbol of the market to come back empty is what keeps that true.
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
	// This is the whole reason the watchlist moved out of startup settings: a symbol
	// added while the system runs is fetched by the very next round.
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
	// Starting up on a Saturday: the whole weekend is in reach and none of it could
	// hold a candle, so only Friday's session is asked for.
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
	// Sunday morning, with the lookback covering only the weekend. There is no gap
	// here — the market was shut — so there is nothing to ask for and nothing to
	// report as missing.
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
	// The evening of a trading day. A scheduled round has nothing left to collect,
	// which is exactly when somebody wants today's candles for a stock they just
	// added — so the on-demand catch-up must still reach back into the session.
	//
	// It reaches into the *previous* session too, and that follows from where a
	// backfill starts: reaching back a day from 20:00 lands mid-day, so it starts at
	// that day's edge instead — which puts the whole of the previous trading session
	// inside the window rather than just past its close. Asking for a stretch and
	// then skipping a session that falls inside it would leave a hole the next round
	// never comes back for.
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
	// A chart can be opened for a symbol that is not on the watchlist, and catching
	// that one up is precisely what somebody looking at it is asking for. Reaching it
	// through the watchlist would refuse the request it exists to serve.
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
	// Without a registration there is no market, and without a market there is no
	// source to ask. Guessing one from the shape of the name is the rule this system
	// deliberately lacks.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "9999").
		Return(entities.TradingSymbol{}, false, nil)

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "9999")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNotRegistered)
}

func TestCatchingUpNothingIsRefusedAsAName(t *testing.T) {
	// Blank is not a symbol nobody registered — it is not a symbol at all, and the
	// two ask opposite things of whoever asked: register it, or retype it.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T20:00:00+08:00"))

	_, catchUpError := underTest.service.RunBackfillFor(t.Context(), "   ")

	assert.ErrorIs(t, catchUpError, domains.ErrTradingSymbolNamed)
	assert.NotErrorIs(t, catchUpError, domains.ErrTradingSymbolNotRegistered)
}

func TestCatchingOneSymbolUpNeverDecidesItsWholeMarketIsShut(t *testing.T) {
	// One symbol answering with nothing is one symbol's silence. Reading it as the
	// market's would stop every other symbol of that market being fetched for the
	// rest of the day — on the strength of a single button press.
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

	// The scheduled round that follows still asks the source, which it would not do
	// for a market it had decided was shut for the day.
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2454")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)

	report, roundError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, roundError)
	assert.True(t, reportFor(t, report, "2454").WasAsked)
}

func TestCatchingOneSymbolUpAsksEvenWhenItsMarketWasDecidedShut(t *testing.T) {
	// Deciding a market shut is an inference, and an inference can be wrong — a source
	// that publishes late empties every symbol at once, which is the same shape as a
	// holiday. Somebody asking by hand is somebody saying they want the source asked,
	// so their request has to be the way out. Obeying the decision instead would hand
	// them a report saying nothing was collected, which reads exactly like a market
	// that genuinely had nothing.
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
	// Three minutes after the bell a round asks about five candles, and a source that
	// publishes them a moment late empties every symbol at once. Latching a holiday on
	// that costs the market the rest of its day, so silence has to be worth something
	// first: at least as much of the session behind us as the round asked about.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-08T09:03:00+08:00"))
	underTest.watchingInMarket(vo.MarketTaiwanStock, "2330")
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)
	_, roundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, roundError)

	// Later the same morning the source has caught up. A market decided shut at 09:03
	// would never be asked again today.
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
	// The watchlist is read once, at the top of the round. A change arriving while
	// symbols are still being fetched belongs to the next round — a round that picked
	// up new symbols halfway through would fetch some of them with the previous
	// round's idea of "now".
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
			// The watchlist "changes" while this round is still fetching. No second
			// read is expected above, so a round that looked again would fail here.
			close(changedMidRound)

			return []vo.MarketKCandleVo{}, nil
		})

	report, runError := underTest.service.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	<-changedMidRound
	assert.Len(t, report.SymbolReports, 1)
}

// historyCeilingDays is the ceiling the history-sync cases are written against.
const historyCeilingDays = 90

func historySyncOf(symbol string, lookbackDays int) dto.KCandleHistorySyncDto {
	return dto.KCandleHistorySyncDto{Symbol: symbol, LookbackDays: lookbackDays}
}

func TestSyncingHistoryAsksForTheWholeStretchTheCallerNamed(t *testing.T) {
	// The window's start comes from the lookback and nothing else. That is the whole
	// difference from a backfill, which starts wherever the stored data left off.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	askedWindows := underTest.recordEveryWindowAskedAbout(
		[]vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))})

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	// The stretch is covered a day at a time, so what matters is that the pieces
	// together span exactly what was asked for and leave no minute between them.
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
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, len(*askedWindows), report.SymbolReports[0].StoredCount)
}

// recordEveryWindowAskedAbout answers every chunk with the same candles and keeps the
// windows in the order they were asked for, so a case can check the stretch they
// cover between them.
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

func TestSyncingHistoryAsksAboutTheWholeStretchEvenWhereItAlreadyHasData(t *testing.T) {
	// A backfill starts after the newest candle it holds, so a hole in the middle of
	// a stretch is one it never comes back for. This asks about the whole stretch, and
	// that is what lets a hole be filled at all — the store is not consulted to narrow
	// the window.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	askedWindows := underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.NotEmpty(t, *askedWindows)
	assert.Equal(t, time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), (*askedWindows)[0].StartTime)
}

func TestSyncingHistorySkipsAStretchItAlreadyHoldsEveryCandleOf(t *testing.T) {
	// Over a long stretch nearly every day is already complete, and asking the source
	// about one is a round trip that can only answer with what is already held. The
	// counting read that establishes it costs a fraction of that.
	//
	// This is the one place stored data is consulted, and it decides whether to ask —
	// never where to start. Starting from what is stored is what leaves a hole in the
	// middle unreachable, which is the whole reason this use case exists.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)

	// The first whole day back is complete; nothing else is.
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

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.NotEmpty(t, *askedWindows)
	for _, askedWindow := range *askedWindows {
		assert.False(t, askedWindow.StartTime.Equal(completeDay),
			"已經齊全的那一天不應該再去問來源")
	}
}

func TestSyncingHistoryKeepsWhatItAlreadyStoredWhenTheSourceGivesUpPartWay(t *testing.T) {
	// Storing chunk by chunk is what makes a long stretch survivable: a source that
	// stops answering halfway leaves the first half stored and correct, and running
	// again picks up from there because nothing already held is written twice.
	//
	// The rest of the stretch is abandoned rather than attempted, because a source
	// that just refused one chunk will refuse the next thousand the same way.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
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

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	// The first chunk is stored before the second is even asked for.
	assert.Equal(t, 1, storedBatches)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "unreachable")
	// It stopped at the refusal instead of walking the rest of the stretch into it.
	assert.Equal(t, 2, askedTimes)
}

func TestSyncingHistoryCountsEverySkippedKCandleButOnlyNamesSoMany(t *testing.T) {
	// A source answering with rubbish for years turns an unbounded list into a report
	// nobody can open. The count stays honest either way.
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

	brokenKCandles := make([]vo.MarketKCandleVo, 0, 300)
	for minute := range 300 {
		brokenKCandles = append(brokenKCandles,
			reportedKCandle(ingestionAt(0, minute, 0), "90", "120"))
	}
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(brokenKCandles, nil).Times(1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, 300, report.SymbolReports[0].SkippedCount)
	assert.Len(t, report.SymbolReports[0].SkippedKCandles, 200)
	assert.True(t, report.SymbolReports[0].SkippedKCandlesTruncated)
}

func TestSyncingHistoryFailsOutrightWhenStorageBreaks(t *testing.T) {
	// Storage breaking is this system's own fault rather than anything the candles
	// did, so it ends the request instead of being written down as a skipped candle.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		Return(0, errors.New("storage unavailable")).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{validReportedKCandle(ingestionAt(9, 5, 0))}, nil).AnyTimes()

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.Error(t, syncError)
}

func TestSyncingHistoryLeavesAlreadyStoredKCandlesAlone(t *testing.T) {
	// It fills in what is missing. A candle already held is not written over, which is
	// what separates this from every other path here: the scheduled round legitimately
	// replaces a candle it collected while it was still forming, and this must not.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
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

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	// The count is what was newly stored. Everything already held is silently left
	// as it was — a stretch that was complete already reports nothing stored, which
	// is the truth about what this run did.
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].SkippedKCandles)
}

func TestTheOtherIngestionPathsStillOverwriteWhatTheyCollected(t *testing.T) {
	// A scheduled round collects the candle of the minute that just closed, and the
	// next round collects it again once the source has settled its figures. Refusing
	// to overwrite there would freeze the first, roughest version of every candle.
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
			// Refused before the store is touched: a request that cannot work should
			// not read anything first.
			underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
			underTest.tradingSymbolRepository.EXPECT().
				FindBySymbol(gomock.Any(), gomock.Any()).Times(0)

			_, syncError := underTest.service.SyncHistoryFor(
				t.Context(), historySyncOf("BTCUSDT", testCase.lookbackDays), historyCeilingDays)

			assert.ErrorIs(t, syncError, domains.ErrKCandleHistoryLookback)
		})
	}
}

func TestSyncingHistoryRefusesASymbolNobodyRegistered(t *testing.T) {
	// Same rule as the on-demand catch-up: without a registration there is no market,
	// and without a market there is no source to ask.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "9999").
		Return(entities.TradingSymbol{}, false, nil)

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("9999", 2), historyCeilingDays)

	assert.ErrorIs(t, syncError, domains.ErrTradingSymbolNotRegistered)
}

func TestSyncingHistoryRefusesNothingAsAName(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("   ", 2), historyCeilingDays)

	assert.ErrorIs(t, syncError, domains.ErrTradingSymbolNamed)
	assert.NotErrorIs(t, syncError, domains.ErrTradingSymbolNotRegistered)
}

func TestSyncingHistoryReachesASymbolNobodyIsWatching(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: false,
		}, true, nil)
	underTest.recordEveryWindowAskedAbout([]vo.MarketKCandleVo{})

	_, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
}

func TestSyncingHistoryReportsTheCandlesItCouldNotStore(t *testing.T) {
	// The report is the same shape a backfill produces, down to naming the candle
	// that broke a rule. One thing should not have two ways of being described.
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.syncingFromEmptyStorage()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{
			validReportedKCandle(ingestionAt(9, 4, 0)),
			reportedKCandle(ingestionAt(9, 5, 0), "90", "120"),
		}, nil).Times(1)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil).AnyTimes()

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	assert.True(t, report.SymbolReports[0].WasAsked)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	require.Len(t, report.SymbolReports[0].SkippedKCandles, 1)
	assert.Equal(t, ingestionAt(9, 5, 0), report.SymbolReports[0].SkippedKCandles[0].OpenTime)
}

func TestSyncingHistoryReportsASourceThatWouldNotAnswer(t *testing.T) {
	underTest := newIngestionUnderTest(t, ingestionAt(9, 7, 30))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable)

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("BTCUSDT", 2), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "unreachable")
}

func TestSyncingHistoryDropsAStandingDecisionThatTheMarketIsShut(t *testing.T) {
	// Somebody asking by hand is somebody saying they want the source asked. Obeying
	// a presumed holiday would answer them with a report saying nothing was
	// collected — indistinguishable from a market that genuinely had nothing — and
	// leave them no way to correct a decision that may have been wrong.
	//
	// Same rule as the on-demand catch-up, and the same code; this pins that going in
	// through the history sync reaches it too.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-11T10:00:00+08:00"))
	underTest.syncingFromEmptyStorage()
	underTest.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.TradingSymbol{{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}}, nil).AnyTimes()
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil).AnyTimes()

	// A scheduled round that hears nothing from every watched symbol of a market is
	// what earns the "shut for the day" conclusion in the first place.
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)
	_, roundError := underTest.service.RunScheduledRound(t.Context())
	require.NoError(t, roundError)

	// The next scheduled round obeys that decision and never reaches the source.
	// The history sync, arriving right after it, must.
	askedAgain := false
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			askedAgain = true

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("2330", 2), historyCeilingDays)

	require.NoError(t, syncError)
	assert.True(t, askedAgain, "手動同步必須真的去問來源，而不是沿用「今天休市」那個判斷")
	require.Len(t, report.SymbolReports, 1)
	assert.True(t, report.SymbolReports[0].WasAsked)
}

func TestSyncingHistoryOverAClosedMarketIsNotAFailure(t *testing.T) {
	// A stretch that lies entirely outside a market's session holds no candle, and
	// that is a fact about the market rather than a failed request. It has to be
	// told apart from a source that would not answer: one is normal, the other is
	// worth chasing.
	underTest := newIngestionUnderTest(t, taipeiIngestionAt(t, "2026-09-13T10:00:00+08:00"))
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
		entities.TradingSymbol{
			Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true,
		}, true, nil)
	// The whole window falls on a weekend, so the source is never even reached.
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	report, syncError := underTest.service.SyncHistoryFor(
		t.Context(), historySyncOf("2330", 1), historyCeilingDays)

	require.NoError(t, syncError)
	require.Len(t, report.SymbolReports, 1)
	assert.False(t, report.SymbolReports[0].WasAsked)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
}
