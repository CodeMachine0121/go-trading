package job_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testInterval     = 20 * time.Millisecond
	roundCandleCount = 5
	lookback         = 24 * time.Hour
	currentHour      = 9
	currentMinute    = 7
)

var currentTime = time.Date(2026, 8, 30, currentHour, currentMinute, 0, 0, time.UTC)

// scheduledWindowStart is where a periodic round begins: four candles back from the
// newest closed one.
var scheduledWindowStart = time.Date(2026, 8, 30, 8, 40, 0, 0, time.UTC)

type jobUnderTest struct {
	job             *job.KCandleIngestionJob
	stages          chan string
	backfillSymbols chan string
}

// newJobUnderTest builds the real ingestion path and records which half of the job
// reached the outside world, in the order it happened.
func newJobUnderTest(t *testing.T, symbols []string) jobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	stages := make(chan string, 64)
	backfillSymbols := make(chan string, 64)

	kCandleRepository.EXPECT().FindLatest(gomock.Any(), 1).
		DoAndReturn(func(symbol string, limit int) ([]entities.KCandle, error) {
			stages <- "backfill"
			backfillSymbols <- symbol
			return []entities.KCandle{}, nil
		}).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).
		DoAndReturn(func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			if window.StartTime.Equal(scheduledWindowStart) {
				stages <- "scheduled round"
			}
			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	ingestionJob := job.NewKCandleIngestionJob(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, marketDataProxy, clockProxy, roundCandleCount, lookback)),
		symbols, testInterval)
	t.Cleanup(ingestionJob.Stop)

	return jobUnderTest{job: ingestionJob, stages: stages, backfillSymbols: backfillSymbols}
}

func nextFrom(t *testing.T, events chan string) string {
	t.Helper()

	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("the job produced nothing in time")
		return ""
	}
}

func TestTheJobBackfillsBeforeItStartsKeepingUp(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})

	underTest.job.Start()

	assert.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheJobKeepsRunningRoundsAtItsInterval(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})

	underTest.job.Start()

	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheJobWorksFromItsOwnCopyOfTheWatchlist(t *testing.T) {
	watchlist := []string{"BTCUSDT"}
	underTest := newJobUnderTest(t, watchlist)
	watchlist[0] = "SOLUSDT"

	underTest.job.Start()

	assert.Equal(t, "BTCUSDT", nextFrom(t, underTest.backfillSymbols))
}

func TestAStoppedJobRunsNoFurtherRounds(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})
	underTest.job.Start()
	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	require.Equal(t, "scheduled round", nextFrom(t, underTest.stages))

	underTest.job.Stop()
	time.Sleep(5 * testInterval)
	drain(underTest.stages)
	time.Sleep(5 * testInterval)

	assert.Empty(t, underTest.stages)
}

func drain(events chan string) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}
