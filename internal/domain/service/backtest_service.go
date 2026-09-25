package service

import (
	"context"
	"errors"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// BacktestService is the application layer's only entry point for replaying strategy scripts over past market data; it only sequences the domain steps.
type BacktestService struct {
	kCandleRepository    domaininterface.IKCandleRepository
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy
	clockProxy           domaininterface.IClockProxy
	maxCandleCount       int
	// replayTimeAllowance bounds one whole replay, reading plus all script runs, on top of each run's own allowance.
	replayTimeAllowance time.Duration
}

func NewBacktestService(
	kCandleRepository domaininterface.IKCandleRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
	replayTimeAllowance time.Duration,
) *BacktestService {
	return &BacktestService{
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
		clockProxy:           clockProxy,
		maxCandleCount:       maxCandleCount,
		replayTimeAllowance:  replayTimeAllowance,
	}
}

// RunBacktest replays the script over every finished candle, oldest first, each run seeing all candles up to its own; nothing is stored.
func (backtestService *BacktestService) RunBacktest(
	executionContext context.Context, requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	// The allowance covers reading the market as well as running the scripts.
	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, backtestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

	backtestDomain, validationError := domains.NewBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())
	if validationError != nil {
		return dto.BacktestResultDto{}, validationError
	}

	kCandles, findError := backtestService.kCandleRepository.FindInRange(
		replayContext, backtestDomain.KCandleQuery(), backtestDomain.SourceCandleLimit())
	if findError != nil {
		return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, findError)
	}

	inputKCandles, selectionError := backtestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, selectionError)
	}

	perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
		replayContext,
		requestDto.Script,
		backtestDomain.ResultType(),
		inputKCandles,
		backtestDomain.Parameters())
	if executionError != nil {
		return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, executionError)
	}

	return backtestDomain.ReplayOver(
		inputKCandles, signalsOf(perCandleIndicatorValues), nil), nil
}

// RunTradingStrategyBacktest replays a whole trading strategy; candles are read once so every source sees the identical stretch.
func (backtestService *BacktestService) RunTradingStrategyBacktest(
	executionContext context.Context, requestDto dto.TradingStrategyBacktestRequestDto,
) (dto.BacktestResultDto, error) {
	// The allowance covers reading the market as well as running the scripts.
	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, backtestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

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
		return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, findError)
	}

	inputKCandles, selectionError := tradingStrategyBacktestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, selectionError)
	}

	signalsBySource := make([][]domains.SignalDomain, 0, tradingStrategyBacktestDomain.SourceCount())
	for sourceIndex := range tradingStrategyBacktestDomain.SourceCount() {
		perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
			replayContext,
			tradingStrategyBacktestDomain.SourceScript(sourceIndex),
			tradingStrategyBacktestDomain.ResultType(),
			inputKCandles,
			tradingStrategyBacktestDomain.SourceParameters(sourceIndex))
		// One failing source ends the replay; missing signals would silently make conditions false.
		if executionError != nil {
			return dto.BacktestResultDto{}, backtestService.refusalFor(replayContext, executionError)
		}

		signalsBySource = append(signalsBySource, signalsOf(perCandleIndicatorValues))
	}

	return tradingStrategyBacktestDomain.ReplayOver(inputKCandles, signalsBySource), nil
}

// errReplayTimeAllowanceSpent distinguishes the replay's own allowance running out from the caller giving up.
var errReplayTimeAllowanceSpent = errors.New("replay time allowance spent")

// refusalFor explains an unfinished replay, with an actionable message when the allowance ran out.
func (backtestService *BacktestService) refusalFor(replayContext context.Context, executionError error) error {
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return domains.BacktestTimeAllowanceSpent(backtestService.replayTimeAllowance)
	}

	return executionError
}

func signalsOf(perCandleIndicatorValues []map[string]vo.IndicatorValueVo) []domains.SignalDomain {
	signals := make([]domains.SignalDomain, 0, len(perCandleIndicatorValues))
	for _, indicatorValues := range perCandleIndicatorValues {
		signals = append(signals, domains.NewSignalDomain(indicatorValues))
	}

	return signals
}
