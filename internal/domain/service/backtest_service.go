package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// BacktestService is the application layer's only entry point for replaying strategy scripts over past market data; it only sequences the domain steps.
type BacktestService struct {
	kCandleRepository     domaininterface.IKCandleRepository
	indicatorScriptProxy  domaininterface.IIndicatorScriptProxy
	clockProxy            domaininterface.IClockProxy
	maxCandleCount        int
	defaultWarmupBarCount int
}

func NewBacktestService(
	kCandleRepository domaininterface.IKCandleRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
	defaultWarmupBarCount int,
) *BacktestService {
	return &BacktestService{
		kCandleRepository:     kCandleRepository,
		indicatorScriptProxy:  indicatorScriptProxy,
		clockProxy:            clockProxy,
		maxCandleCount:        maxCandleCount,
		defaultWarmupBarCount: defaultWarmupBarCount,
	}
}

// ValidateBacktest judges a request exactly as a replay would before reading any candle, so nothing invalid is ever queued.
func (backtestService *BacktestService) ValidateBacktest(requestDto dto.BacktestRequestDto) error {
	_, validationError := domains.NewBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())

	return validationError
}

func (backtestService *BacktestService) ValidateTradingStrategyBacktest(
	requestDto dto.TradingStrategyBacktestRequestDto,
) error {
	_, validationError := domains.NewTradingStrategyBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())

	return validationError
}

// RunBacktest replays the script over every finished candle, oldest first, each run seeing all candles up to its own; progress, when a sink is given, is reported per candle.
func (backtestService *BacktestService) RunBacktest(
	executionContext context.Context,
	requestDto dto.BacktestRequestDto,
	progress domaininterface.IReplayProgressSink,
) (dto.BacktestResultDto, error) {
	allowance := domains.NewReplayAllowanceDomain(requestDto.TimeAllowance)
	replayContext, stopReplaying := allowance.Bound(executionContext)
	defer stopReplaying()

	if requestDto.DefaultWarmupBarCount == 0 {
		requestDto.DefaultWarmupBarCount = backtestService.defaultWarmupBarCount
	}
	backtestDomain, validationError := domains.NewBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())
	if validationError != nil {
		return dto.BacktestResultDto{}, validationError
	}

	kCandles, findError := backtestService.kCandleRepository.FindInRange(
		replayContext, backtestDomain.KCandleQuery(), backtestDomain.SourceCandleLimit())
	if findError != nil {
		return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, findError)
	}

	inputKCandles, selectionError := backtestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, selectionError)
	}

	perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
		replayContext,
		requestDto.Script,
		backtestDomain.ResultType(),
		inputKCandles,
		backtestDomain.Parameters(),
		progress)
	if executionError != nil {
		return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, executionError)
	}

	return backtestDomain.ReplayOver(
		inputKCandles, signalsOf(perCandleIndicatorValues), nil), nil
}

// RunTradingStrategyBacktest replays a whole trading strategy; candles are read once so every source sees the identical stretch, and progress counts every source's bars.
func (backtestService *BacktestService) RunTradingStrategyBacktest(
	executionContext context.Context,
	requestDto dto.TradingStrategyBacktestRequestDto,
	progress domaininterface.IReplayProgressSink,
) (dto.BacktestResultDto, error) {
	allowance := domains.NewReplayAllowanceDomain(requestDto.TimeAllowance)
	replayContext, stopReplaying := allowance.Bound(executionContext)
	defer stopReplaying()

	if requestDto.DefaultWarmupBarCount == 0 {
		requestDto.DefaultWarmupBarCount = backtestService.defaultWarmupBarCount
	}
	tradingStrategyBacktestDomain, validationError := domains.NewTradingStrategyBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())
	if validationError != nil {
		return dto.BacktestResultDto{}, validationError
	}

	kCandles, findError := backtestService.kCandleRepository.FindInRange(
		replayContext,
		tradingStrategyBacktestDomain.KCandleQuery(),
		tradingStrategyBacktestDomain.SourceCandleLimit())
	if findError != nil {
		return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, findError)
	}

	inputKCandles, selectionError := tradingStrategyBacktestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, selectionError)
	}

	sourceCount := tradingStrategyBacktestDomain.SourceCount()
	signalsBySource := make([][]domains.SignalDomain, 0, sourceCount)
	for sourceIndex := range sourceCount {
		perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
			replayContext,
			tradingStrategyBacktestDomain.SourceScript(sourceIndex),
			tradingStrategyBacktestDomain.ResultType(),
			inputKCandles,
			tradingStrategyBacktestDomain.SourceParameters(sourceIndex),
			newSourceReplayProgress(progress, sourceIndex, sourceCount))
		// One failing source ends the replay; missing signals would silently make conditions false.
		if executionError != nil {
			return dto.BacktestResultDto{}, allowance.RefusalFor(replayContext, executionError)
		}

		signalsBySource = append(signalsBySource, signalsOf(perCandleIndicatorValues))
	}

	return tradingStrategyBacktestDomain.ReplayOver(inputKCandles, signalsBySource), nil
}

func signalsOf(perCandleIndicatorValues []map[string]vo.IndicatorValueVo) []domains.SignalDomain {
	signals := make([]domains.SignalDomain, 0, len(perCandleIndicatorValues))
	for _, indicatorValues := range perCandleIndicatorValues {
		signals = append(signals, domains.NewSignalDomain(indicatorValues))
	}

	return signals
}
