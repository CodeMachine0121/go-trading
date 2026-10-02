package job_test

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
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// taiwanSessionMoment is a Monday 10:00 in Taipei, inside the trading session.
var taiwanSessionMoment = time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)

type rosterJobUnderTest struct {
	job           *job.LiveFollowRosterJob
	followService *service.KCandleFollowService
	rosterReads   chan string
	// sourceOpened and sourceEnded tick when a line to the source is opened and when it is let go.
	sourceOpened chan string
	sourceEnded  chan string
}

func newRosterJobUnderTest(t *testing.T, jobLeadershipApplication *application.JobLeadershipApplication) rosterJobUnderTest {
	t.Helper()

	return newRosterJobUnderTestEvery(t, jobLeadershipApplication, testInterval)
}

// newRosterJobUnderTestEvery looks at the duty every test interval but refreshes the roster only every interval.
func newRosterJobUnderTestEvery(
	t *testing.T, jobLeadershipApplication *application.JobLeadershipApplication, interval time.Duration,
) rosterJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	rosterReads := make(chan string, 256)
	sourceOpened := make(chan string, 16)
	sourceEnded := make(chan string, 16)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(executionContext context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			sourceOpened <- channel.Key
			go func() {
				<-executionContext.Done()
				sourceEnded <- channel.Key
			}()

			return make(chan vo.LiveKCandleVo), nil
		}).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.TradingSymbol, error) {
			rosterReads <- "roster"

			return []entities.TradingSymbol{
				{Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true},
			}, nil
		}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(taiwanSessionMoment).AnyTimes()
	followService := service.NewKCandleFollowService(
		liveMarketDataProxy, mocks.NewMockIKCandleRepository(mockController), tradingSymbolRepository, clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
			vo.MarketTaiwanStock: {
				TradingSession: vo.TradingSessionVo{
					Location:   time.FixedZone("Asia/Taipei", 8*60*60),
					DailyStart: 9 * time.Hour,
					DailyEnd:   13*time.Hour + 30*time.Minute,
					Weekdays:   []time.Weekday{time.Monday},
				},
				FollowsFixedRoster:    true,
				SymbolsPerLiveChannel: 25,
			},
		}),
		time.Nanosecond, time.Hour, time.Hour, allowingSnapshots(t), time.Second)
	t.Cleanup(followService.Stop)

	rosterJob := job.NewLiveFollowRosterJob(
		application.NewKCandleFollowApplication(followService), jobLeadershipApplication, interval, testInterval)
	t.Cleanup(rosterJob.Stop)

	return rosterJobUnderTest{
		job: rosterJob, followService: followService, rosterReads: rosterReads,
		sourceOpened: sourceOpened, sourceEnded: sourceEnded,
	}
}

func TestTheRosterJobFollowsTheRosterOnDuty(t *testing.T) {
	underTest := newRosterJobUnderTest(t, onDuty(t))

	underTest.job.Start(t.Context())

	assert.Equal(t, "roster", nextFrom(t, underTest.rosterReads))
	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 1 },
		time.Second, 5*time.Millisecond)
}

func TestTheRosterJobRelaysTheRosterOffDutyWithoutAskingTheSource(t *testing.T) {
	underTest := newRosterJobUnderTest(t, newDuty(t, false).application)

	underTest.job.Start(t.Context())

	assert.Equal(t, "roster", nextFrom(t, underTest.rosterReads))
	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 1 },
		time.Second, 5*time.Millisecond, "viewers on a replica off duty still have a live follow to join")
	assert.Empty(t, underTest.sourceOpened, "the source's places belong to the replica on duty")
}

func TestTheRosterJobLetsGoOfTheSourceAtTheNextDutyCheckAfterLeavingDuty(t *testing.T) {
	duty := newDuty(t, true)
	underTest := newRosterJobUnderTestEvery(t, duty.application, time.Hour)
	underTest.job.Start(t.Context())
	openedKey := nextFrom(t, underTest.sourceOpened)

	duty.waitForChecks(t, 3)
	drain(underTest.rosterReads)
	assert.Empty(t, underTest.rosterReads, "within the interval the roster is not read again")

	duty.set(t, false)
	assert.Equal(t, openedKey, nextFrom(t, underTest.sourceEnded),
		"leaving duty lets go of the source at the next duty check, not the next interval")

	duty.set(t, true)
	assert.Equal(t, openedKey, nextFrom(t, underTest.sourceOpened), "coming back on duty follows the source at once")
}

// allowingSnapshots is a snapshot store that accepts every live candle passed on and has none to relay.
func allowingSnapshots(t *testing.T) *mocks.MockILiveKCandleSnapshotRepository {
	t.Helper()

	snapshotRepository := mocks.NewMockILiveKCandleSnapshotRepository(gomock.NewController(t))
	snapshotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	snapshotRepository.EXPECT().FindObservedAfter(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	snapshotRepository.EXPECT().DeleteObservedBefore(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return snapshotRepository
}
