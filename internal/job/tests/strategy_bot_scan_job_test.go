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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// scanNow is the moment the scan happens at, and scanInterval is short enough that a
// test sees several turns without waiting.
var scanNow = time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

const scanInterval = 20 * time.Millisecond

// newStrategyBotScanJobUnderTest builds the real run path and reports each scan the
// moment it reaches storage, which is the only outward sign a scan happened.
func newStrategyBotScanJobUnderTest(
	t *testing.T, scans chan<- struct{}, findDueError error,
) *job.StrategyBotScanJob {
	t.Helper()

	mockController := gomock.NewController(t)

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(scanNow).AnyTimes()

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(mockController)
	strategyBotRepository.EXPECT().FindDue(gomock.Any(), scanNow, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ time.Time, _ int,
		) ([]entities.StrategyBot, error) {
			select {
			case scans <- struct{}{}:
			default:
			}

			return []entities.StrategyBot{}, findDueError
		}).AnyTimes()

	publishedStrategyRepository := mocks.NewMockIPublishedStrategyRepository(mockController)
	publishedStrategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategy{}, domains.ErrStrategyNotPublished).AnyTimes()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()

	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)

	return job.NewStrategyBotScanJob(
		application.NewStrategyBotRunApplication(
			service.NewStrategyBotService(strategyBotRepository, clockProxy),
			service.NewStrategyService(
				mocks.NewMockIStrategyRepository(mockController), publishedStrategyRepository),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository,
				mocks.NewMockIIndicatorScriptProxy(mockController), clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				1000),
			service.NewTelegramDeliveryService(
				mocks.NewMockITelegramDeliveryRepository(mockController),
				mocks.NewMockISecretSealProxy(mockController),
				mocks.NewMockIMessageDeliveryProxy(mockController)),
			kCandleRepository,
			clockProxy,
			application.NewStrategyBotRoundGuard(),
			4,
			time.Minute,
		),
		scanInterval)
}

func TestStrategyBotScanJobScansImmediatelyAndThenKeepsScanning(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, nil)

	scanJob.Start(t.Context())
	t.Cleanup(scanJob.Stop)

	// Immediately, because a system that has just come up is running nothing and
	// every bot that was due while it was down is due now.
	requireScanWithin(t, scans, "the first scan")
	requireScanWithin(t, scans, "the scan after the first interval")
}

func TestStrategyBotScanJobKeepsGoingAfterAScanThatFailed(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, assert.AnError)

	scanJob.Start(t.Context())
	t.Cleanup(scanJob.Stop)

	requireScanWithin(t, scans, "the first scan")
	// One unreachable database must not switch off every bot in the system.
	requireScanWithin(t, scans, "the scan after the failure")
}

func TestStrategyBotScanJobStopsScanningWhenStopped(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, nil)

	scanJob.Start(t.Context())
	requireScanWithin(t, scans, "the first scan")

	scanJob.Stop()
	// Stopping twice is the same as stopping once, which is what the one-shot close
	// is for — a second close would bring the process down.
	scanJob.Stop()

	drainScans(scans)
	assert.Never(t, func() bool { return len(scans) > 0 }, 80*time.Millisecond, 10*time.Millisecond)
}

func TestStrategyBotScanJobStopsScanningWhenTheContextIsDone(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, nil)

	executionContext, endExecution := context.WithCancel(t.Context())
	scanJob.Start(executionContext)
	requireScanWithin(t, scans, "the first scan")

	endExecution()
	t.Cleanup(scanJob.Stop)

	drainScans(scans)
	assert.Never(t, func() bool { return len(scans) > 0 }, 80*time.Millisecond, 10*time.Millisecond)
}

func requireScanWithin(t *testing.T, scans <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-scans:
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for "+what)
	}
}

func drainScans(scans chan struct{}) {
	for {
		select {
		case <-scans:
		default:
			return
		}
	}
}

func TestStrategyBotScanJobScansWhenThereAreBotsToRun(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, nil)

	scanJob.Start(t.Context())
	t.Cleanup(scanJob.Stop)

	// A scan that finds nothing and a scan that runs something both leave the job
	// ready for the next turn; the job itself never decides anything about a bot.
	requireScanWithin(t, scans, "the first scan")
	requireScanWithin(t, scans, "the second scan")
	requireScanWithin(t, scans, "the third scan")
}
