package job_test

import (
	"context"
	"sync"
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

	kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		DoAndReturn(func(_ context.Context, symbol string, limit int) ([]entities.KCandle, error) {
			stages <- "backfill"
			backfillSymbols <- symbol
			return []entities.KCandle{}, nil
		}).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
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

	underTest.job.Start(t.Context())

	assert.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheJobKeepsRunningRoundsAtItsInterval(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})

	underTest.job.Start(t.Context())

	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
	assert.Equal(t, "scheduled round", nextFrom(t, underTest.stages))
}

func TestTheJobWorksFromItsOwnCopyOfTheWatchlist(t *testing.T) {
	watchlist := []string{"BTCUSDT"}
	underTest := newJobUnderTest(t, watchlist)
	watchlist[0] = "SOLUSDT"

	underTest.job.Start(t.Context())

	assert.Equal(t, "BTCUSDT", nextFrom(t, underTest.backfillSymbols))
}

func TestAStoppedJobRunsNoFurtherRounds(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})
	underTest.job.Start(t.Context())
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

func TestTheIntervalBetweenRoundsIsTheLengthOneKCandleCovers(t *testing.T) {
	assert.Equal(t, 5*time.Minute, job.KCandleIngestionInterval)
}

// The context is the second way a job ends, and it has to work on its own: an
// orderly shutdown reaches for Stop first, but a shutdown that has run out of
// patience has only this.
func TestAJobWhoseContextIsDoneRunsNoFurtherRounds(t *testing.T) {
	underTest := newJobUnderTest(t, []string{"BTCUSDT"})
	backgroundJobWork, giveUpOnBackgroundJobWork := context.WithCancel(t.Context())

	underTest.job.Start(backgroundJobWork)
	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	require.Equal(t, "scheduled round", nextFrom(t, underTest.stages))

	giveUpOnBackgroundJobWork()
	time.Sleep(5 * testInterval)
	drain(underTest.stages)
	time.Sleep(5 * testInterval)

	assert.Empty(t, underTest.stages)
}

// slowJobUnderTest is the same real ingestion path, except that a scheduled round
// does not finish until the test lets it. Holding a round open is what leaves the
// interval free to tick past it, which is the only way a tick comes to be waiting in
// the channel when something else happens.
type slowJobUnderTest struct {
	job             *job.KCandleIngestionJob
	stages          chan string
	releaseTheRound func()
}

func newSlowJobUnderTest(t *testing.T) slowJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	stages := make(chan string, 64)
	heldRound := make(chan struct{})
	releaseTheRound := sync.OnceFunc(func() { close(heldRound) })

	kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		DoAndReturn(func(_ context.Context, _ string, _ int) ([]entities.KCandle, error) {
			stages <- "backfill"
			return []entities.KCandle{}, nil
		}).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			if !window.StartTime.Equal(scheduledWindowStart) {
				return []vo.MarketKCandleVo{}, nil
			}

			stages <- "scheduled round"
			<-heldRound

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	ingestionJob := job.NewKCandleIngestionJob(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, marketDataProxy, clockProxy, roundCandleCount, lookback)),
		[]string{"BTCUSDT"}, testInterval)
	// Released first whatever happens, so a test that fails partway cannot leave the
	// round blocked and the job unable to notice it was stopped.
	t.Cleanup(releaseTheRound)
	t.Cleanup(ingestionJob.Stop)

	return slowJobUnderTest{job: ingestionJob, stages: stages, releaseTheRound: releaseTheRound}
}

// A round that outruns the interval leaves a tick waiting in the ticker's channel,
// so when a stop arrives both cases are ready at once and a select picks between them
// at random. Told to stop, the job must not start one more round anyway — half the
// time was the old answer.
func TestAJobToldToStopStartsNoRoundFromATickThatWasAlreadyWaiting(t *testing.T) {
	underTest := newSlowJobUnderTest(t)

	underTest.job.Start(t.Context())
	require.Equal(t, "backfill", nextFrom(t, underTest.stages))
	require.Equal(t, "scheduled round", nextFrom(t, underTest.stages))

	// The round is held open well past the interval, so a tick is certainly waiting
	// by the time the stop lands. Nothing is drained after this point: a round the
	// job should not have started announces itself on this channel, and draining
	// would be throwing away the evidence.
	time.Sleep(10 * testInterval)
	underTest.job.Stop()
	underTest.releaseTheRound()
	time.Sleep(10 * testInterval)

	assert.Empty(t, underTest.stages, "a job told to stop began another round")
}
