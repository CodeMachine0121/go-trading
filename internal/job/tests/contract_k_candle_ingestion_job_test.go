package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var assertAJobError = errors.New("contract market source unreachable")

type contractJobUnderTest struct {
	job             *job.ContractKCandleIngestionJob
	stages          chan string
	backfillSymbols chan string
}

// newContractJobUnderTest records, in order, which half of the job reached the outside world.
func newContractJobUnderTest(t *testing.T, symbols []string) contractJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()

	stages := make(chan string, 64)
	backfillSymbols := make(chan string, 64)

	candleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		DoAndReturn(func(
			_ context.Context, symbol string, _ int,
		) ([]entities.KCandleContract, error) {
			stages <- "backfill"
			backfillSymbols <- symbol

			return []entities.KCandleContract{}, nil
		}).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, window vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			if window.StartTime.Equal(scheduledWindowStart) {
				stages <- "scheduled round"
			}

			return []vo.ContractMarketKCandleVo{}, nil
		}).AnyTimes()

	watchedSymbols := make([]entities.ContractTradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols,
			entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true})
	}
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchedSymbols, nil).AnyTimes()

	ingestionJob := job.NewContractKCandleIngestionJob(
		application.NewKCandleContractIngestionApplication(
			service.NewContractKCandleIngestionService(
				candleRepository,
				mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController),
				symbolRepository, marketDataProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				roundCandleCount, lookback, nil)),
		testInterval)
	t.Cleanup(ingestionJob.Stop)

	return contractJobUnderTest{
		job: ingestionJob, stages: stages, backfillSymbols: backfillSymbols,
	}
}

func TestTheContractJobBackfillsBeforeItStartsKeepingUp(t *testing.T) {
	// A round overlapping the backfill would have both halves writing the same candle.
	underTest := newContractJobUnderTest(t, []string{"BTCUSDT"})

	underTest.job.Start(t.Context())

	assert.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheContractJobKeepsRunningRoundsAtItsInterval(t *testing.T) {
	underTest := newContractJobUnderTest(t, []string{"BTCUSDT"})

	underTest.job.Start(t.Context())

	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheContractJobWorksFromItsOwnCopyOfTheWatchlist(t *testing.T) {
	watchlist := []string{"BTCUSDT"}
	underTest := newContractJobUnderTest(t, watchlist)
	watchlist[0] = "SOLUSDT"

	underTest.job.Start(t.Context())

	assert.Equal(t, "BTCUSDT", nextFrom(t, underTest.backfillSymbols))
}

func TestAStoppedContractJobRunsNoFurtherRounds(t *testing.T) {
	underTest := newContractJobUnderTest(t, []string{"BTCUSDT"})
	underTest.job.Start(t.Context())
	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	require.Equal(t, "scheduled round", nextFrom(t, underTest.stages))

	underTest.job.Stop()
	time.Sleep(5 * testInterval)
	drainedRounds := len(underTest.stages)
	time.Sleep(5 * testInterval)

	assert.Equal(t, drainedRounds, len(underTest.stages), "停下來之後不該再多跑任何一輪")
}

func TestAContractJobWhoseContextIsDoneRunsNoFurtherRounds(t *testing.T) {
	underTest := newContractJobUnderTest(t, []string{"BTCUSDT"})
	abandonedContext, abandon := context.WithCancel(t.Context())

	underTest.job.Start(abandonedContext)
	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	abandon()
	time.Sleep(5 * testInterval)
	drainedRounds := len(underTest.stages)
	time.Sleep(5 * testInterval)

	assert.Equal(t, drainedRounds, len(underTest.stages))
}

func TestTheContractJobWritesDownWhatWentWrongWithoutStopping(t *testing.T) {
	// One contract failing must not take the round down.
	mockController := gomock.NewController(t)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()
	candleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		Return([]entities.KCandleContract{}, nil).AnyTimes()
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil).AnyTimes()

	rounds := make(chan struct{}, 64)
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			rounds <- struct{}{}

			return nil, assertAJobError
		}).AnyTimes()

	ingestionJob := job.NewContractKCandleIngestionJob(
		application.NewKCandleContractIngestionApplication(
			service.NewContractKCandleIngestionService(
				candleRepository,
				mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController),
				symbolRepository, marketDataProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				roundCandleCount, lookback, nil)),
		testInterval)
	t.Cleanup(ingestionJob.Stop)

	ingestionJob.Start(t.Context())

	// A second round proves the first failure did not end the job.
	for range 2 {
		select {
		case <-rounds:
		case <-time.After(2 * time.Second):
			t.Fatal("來源一直拒絕時，job 應該照樣繼續跑下一輪")
		}
	}
}

func contractJobReaching(
	t *testing.T,
	watchedSymbols func() ([]entities.ContractTradingSymbol, error),
	venueAnswer func() ([]vo.ContractMarketKCandleVo, error),
) *job.ContractKCandleIngestionJob {
	t.Helper()

	mockController := gomock.NewController(t)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()
	candleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		Return([]entities.KCandleContract{}, nil).AnyTimes()
	candleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, nil).AnyTimes()
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	symbolRepository.EXPECT().FindWatched(gomock.Any()).
		DoAndReturn(func(_ context.Context) ([]entities.ContractTradingSymbol, error) {
			return watchedSymbols()
		}).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.KCandleFetchWindowVo,
		) ([]vo.ContractMarketKCandleVo, error) {
			return venueAnswer()
		}).AnyTimes()

	ingestionJob := job.NewContractKCandleIngestionJob(
		application.NewKCandleContractIngestionApplication(
			service.NewContractKCandleIngestionService(
				candleRepository,
				mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController),
				symbolRepository, marketDataProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				roundCandleCount, lookback, nil)),
		testInterval)
	t.Cleanup(ingestionJob.Stop)

	return ingestionJob
}

func TestTheContractJobSurvivesARoundItCouldNotEvenStart(t *testing.T) {
	// An unreadable watchlist ends the round early; the job logs it and carries on.
	readAttempts := make(chan struct{}, 64)
	ingestionJob := contractJobReaching(t,
		func() ([]entities.ContractTradingSymbol, error) {
			readAttempts <- struct{}{}

			return nil, assertAJobError
		},
		func() ([]vo.ContractMarketKCandleVo, error) {
			return []vo.ContractMarketKCandleVo{}, nil
		})

	ingestionJob.Start(t.Context())

	for range 2 {
		select {
		case <-readAttempts:
		case <-time.After(2 * time.Second):
			t.Fatal("讀不到追蹤名單時，job 應該照樣繼續跑下一輪")
		}
	}
}

func TestTheContractJobWritesDownEveryCandleItHadToSkip(t *testing.T) {
	// A minute missing its mark price is skipped and named individually in the log.
	rounds := make(chan struct{}, 64)
	withoutMarkPrice := vo.ContractMarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            time.Date(2026, 8, 30, 9, 5, 0, 0, time.UTC),
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
		TradeCount:          7,
	}
	ingestionJob := contractJobReaching(t,
		func() ([]entities.ContractTradingSymbol, error) {
			return []entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil
		},
		func() ([]vo.ContractMarketKCandleVo, error) {
			rounds <- struct{}{}

			return []vo.ContractMarketKCandleVo{withoutMarkPrice}, nil
		})

	ingestionJob.Start(t.Context())

	for range 2 {
		select {
		case <-rounds:
		case <-time.After(2 * time.Second):
			t.Fatal("跳過整批 K 線時，job 應該照樣繼續跑下一輪")
		}
	}
}
