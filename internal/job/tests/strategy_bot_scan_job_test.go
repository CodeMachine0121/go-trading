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

// scanInterval is short so a test sees several turns quickly.
var scanNow = time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

const scanInterval = 20 * time.Millisecond

// newStrategyBotScanJobUnderTest reports each scan as it reaches storage, the only outward sign of a scan.
func newStrategyBotScanJobUnderTest(
	t *testing.T, scans chan<- struct{}, findDueError error,
) *job.StrategyBotScanJob {
	t.Helper()

	mockController := gomock.NewController(t)

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(scanNow).AnyTimes()

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(mockController)
	// 歷史每一輪都會寫，但寫入成敗不是這些測試關心的事。
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(mockController)
	strategyBotRunRecordRepository.EXPECT().
		Append(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()

	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)

	return job.NewStrategyBotScanJob(
		application.NewStrategyBotRunApplication(
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository,
				mocks.NewMockIContractTradingSymbolRepository(mockController),
				mocks.NewMockIContractMaintenanceMarginTierRepository(mockController),
				mocks.NewMockIContractFundingRateSettlementRepository(mockController),
				clockProxy),
			service.NewTradingStrategyService(
				mocks.NewMockITradingStrategyRepository(mockController)),
			service.NewStrategyScriptService(
				mocks.NewMockIStrategyScriptRepository(mockController), publishedStrategyScriptRepository),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository,
				mocks.NewMockIIndicatorScriptProxy(mockController), clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				1000),
			service.NewContractIndicatorCalculationService(
				mocks.NewMockIKCandleContractRepository(mockController),
				mocks.NewMockIContractFundingRateSettlementRepository(mockController),
				mocks.NewMockIContractPositionStatisticRepository(mockController),
				mocks.NewMockIContractIndicatorScriptProxy(mockController), clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				1000),
			service.NewTelegramDeliveryService(
				mocks.NewMockITelegramDeliveryRepository(mockController),
				mocks.NewMockISecretSealProxy(mockController),
				mocks.NewMockIMessageDeliveryProxy(mockController)),
			service.NewKCandleService(
				kCandleRepository, tradingSymbolRepository, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				1000),
			service.NewKCandleContractService(
				mocks.NewMockIKCandleContractRepository(mockController), clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				1000),
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

	requireScanWithin(t, scans, "the first scan")
	requireScanWithin(t, scans, "the scan after the first interval")
}

func TestStrategyBotScanJobKeepsGoingAfterAScanThatFailed(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, assert.AnError)

	scanJob.Start(t.Context())
	t.Cleanup(scanJob.Stop)

	requireScanWithin(t, scans, "the first scan")
	// One unreachable database must not switch off every bot.
	requireScanWithin(t, scans, "the scan after the failure")
}

func TestStrategyBotScanJobStopsScanningWhenStopped(t *testing.T) {
	scans := make(chan struct{}, 8)
	scanJob := newStrategyBotScanJobUnderTest(t, scans, nil)

	scanJob.Start(t.Context())
	requireScanWithin(t, scans, "the first scan")

	scanJob.Stop()
	// Stop must be idempotent, since a second channel close would panic.
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

	requireScanWithin(t, scans, "the first scan")
	requireScanWithin(t, scans, "the second scan")
	requireScanWithin(t, scans, "the third scan")
}
