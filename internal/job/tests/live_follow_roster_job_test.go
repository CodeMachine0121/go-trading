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
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).
		Return(make(chan vo.LiveKCandleVo), nil).AnyTimes()
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
		time.Nanosecond, time.Hour, time.Hour)
	t.Cleanup(followService.Stop)

	rosterJob := job.NewLiveFollowRosterJob(
		application.NewKCandleFollowApplication(followService), jobLeadershipApplication, interval, testInterval)
	t.Cleanup(rosterJob.Stop)

	return rosterJobUnderTest{job: rosterJob, followService: followService, rosterReads: rosterReads}
}

func TestTheRosterJobFollowsTheRosterOnDuty(t *testing.T) {
	underTest := newRosterJobUnderTest(t, onDuty(t))

	underTest.job.Start(t.Context())

	assert.Equal(t, "roster", nextFrom(t, underTest.rosterReads))
	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 1 },
		time.Second, 5*time.Millisecond)
}

func TestTheRosterJobLetsGoOfTheRosterWhenThisReplicaLeavesDuty(t *testing.T) {
	duty := newDuty(t, true)
	underTest := newRosterJobUnderTest(t, duty.application)
	underTest.job.Start(t.Context())
	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 1 },
		time.Second, 5*time.Millisecond)

	duty.set(t, false)

	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 0 },
		time.Second, 5*time.Millisecond)
	drain(underTest.rosterReads)
	time.Sleep(5 * testInterval)
	assert.Empty(t, underTest.rosterReads, "off duty, the roster is not read at all")
}

func TestTheRosterJobRefreshesOnlyEveryIntervalButLetsGoAtTheNextDutyCheck(t *testing.T) {
	duty := newDuty(t, true)
	underTest := newRosterJobUnderTestEvery(t, duty.application, time.Hour)
	underTest.job.Start(t.Context())
	assert.Equal(t, "roster", nextFrom(t, underTest.rosterReads))

	time.Sleep(10 * testInterval)
	assert.Empty(t, underTest.rosterReads, "within the interval the roster is not read again")

	duty.set(t, false)
	assert.Eventually(t, func() bool { return underTest.followService.FollowedSymbolCount() == 0 },
		time.Second, 5*time.Millisecond, "leaving duty lets go of the roster at the next duty check, not the next interval")

	duty.set(t, true)
	assert.Equal(t, "roster", nextFrom(t, underTest.rosterReads),
		"coming back on duty follows the roster at once")
}
