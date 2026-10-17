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
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// newSummaryJobUnderTest signals every round, which always starts by reading the spot watchlist.
func newSummaryJobUnderTest(
	t *testing.T, jobLeadershipApplication *application.JobLeadershipApplication, watchlistError error,
) seriesJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	rounds := make(chan string, 256)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.TradingSymbol, error) {
			rounds <- "round"

			return nil, watchlistError
		}).AnyTimes()
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, nil).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	summaryJob := job.NewKCandleSummaryJob(
		application.NewKCandleSummaryApplication(service.NewKCandleSummaryService(
			mocks.NewMockIKCandleRepository(mockController), mocks.NewMockIKCandleSummaryDayRepository(mockController),
			mocks.NewMockIKCandleContractRepository(mockController),
			mocks.NewMockIKCandleContractSummaryDayRepository(mockController),
			tradingSymbolRepository, contractTradingSymbolRepository, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}))),
		jobLeadershipApplication, testInterval)
	t.Cleanup(summaryJob.Stop)

	return seriesJobUnderTest{job: summaryJob, rounds: rounds}
}

func TestSummaryJobOnDutyRunsARoundOnStartAndAgainAfterAFailedOne(t *testing.T) {
	underTest := newSummaryJobUnderTest(t, onDuty(t), errors.New("watchlist unreadable"))

	underTest.job.Start(t.Context())

	for range 2 {
		select {
		case <-underTest.rounds:
		case <-time.After(2 * time.Second):
			t.Fatal("the summary job stopped running rounds")
		}
	}
}

func TestSummaryJobOffDutyRunsNoRound(t *testing.T) {
	duty := newDuty(t, false)
	underTest := newSummaryJobUnderTest(t, duty.application, nil)

	underTest.job.Start(t.Context())
	duty.waitForChecks(t, 3)

	assert.Empty(t, underTest.rounds)
}
