package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// BacktestService is the application layer's only entry point for replaying a
// strategy script over a stretch of market that has already happened.
//
// It orchestrates and nothing more: the rules about what may be replayed live in
// BacktestDomain, the rules about trading live in BacktestSimulationDomain, and how a
// script is read and run lives behind the script proxy. What is here is the order the
// four steps happen in, which is the one thing none of them can own.
type BacktestService struct {
	kCandleRepository    domaininterface.IKCandleRepository
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy
	clockProxy           domaininterface.IClockProxy
	maxCandleCount       int
}

func NewBacktestService(
	kCandleRepository domaininterface.IKCandleRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
) *BacktestService {
	return &BacktestService{
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
		clockProxy:           clockProxy,
		maxCandleCount:       maxCandleCount,
	}
}

// RunBacktest replays the script over every finished candle of the requested stretch,
// oldest first, and hands back the report card, the finished round trips and the
// equity curve. Nothing is stored: asking the same question twice replays it twice.
//
// The script is run once per candle and sees everything from the first candle up to
// the one it stands on — so a strategy script that looks back further than it has candles
// simply produces nothing to act on early in the replay, exactly as it would have at
// the time.
func (backtestService *BacktestService) RunBacktest(
	executionContext context.Context, requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	backtestDomain, validationError := domains.NewBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())
	if validationError != nil {
		return dto.BacktestResultDto{}, validationError
	}

	kCandles, findError := backtestService.kCandleRepository.FindInRange(
		executionContext, backtestDomain.KCandleQuery(), backtestDomain.SourceCandleLimit())
	if findError != nil {
		return dto.BacktestResultDto{}, findError
	}

	inputKCandles, selectionError := backtestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, selectionError
	}

	perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
		executionContext,
		requestDto.Script,
		backtestDomain.ResultType(),
		inputKCandles,
		backtestDomain.Parameters())
	if executionError != nil {
		return dto.BacktestResultDto{}, executionError
	}

	// The script ran under the signal kind, so each candle's result carries one
	// opinion. Reading them into signals here keeps the simulation working in
	// opinions rather than in raw script output.
	return backtestDomain.ReplayOver(
		inputKCandles, signalsOf(perCandleIndicatorValues)), nil
}

// RunTradingStrategyBacktest replays a whole trading strategy over the same stretch:
// every signal source runs its own script over the same candles, and the two
// condition trees turn each candle's several opinions into one.
//
// The candles are read once and every source runs over that one batch. Reading per
// source would be the same query repeated up to ten times, and — worse — ten chances
// for two sources to end up replaying slightly different stretches.
//
// It shares no step with RunBacktest beyond the private reading of a script result
// into signals. What replays a single script must keep working exactly as it does,
// and the surest way to keep it that way is for this not to touch it.
func (backtestService *BacktestService) RunTradingStrategyBacktest(
	executionContext context.Context, requestDto dto.TradingStrategyBacktestRequestDto,
) (dto.BacktestResultDto, error) {
	tradingStrategyBacktestDomain, validationError := domains.NewTradingStrategyBacktestDomain(
		requestDto, backtestService.maxCandleCount, backtestService.clockProxy.Now())
	if validationError != nil {
		return dto.BacktestResultDto{}, validationError
	}

	kCandles, findError := backtestService.kCandleRepository.FindInRange(
		executionContext,
		tradingStrategyBacktestDomain.KCandleQuery(),
		tradingStrategyBacktestDomain.SourceCandleLimit())
	if findError != nil {
		return dto.BacktestResultDto{}, findError
	}

	inputKCandles, selectionError := tradingStrategyBacktestDomain.SelectInputCandles(kCandles)
	if selectionError != nil {
		return dto.BacktestResultDto{}, selectionError
	}

	signalsBySource := make([][]domains.SignalDomain, 0, tradingStrategyBacktestDomain.SourceCount())
	for sourceIndex := range tradingStrategyBacktestDomain.SourceCount() {
		perCandleIndicatorValues, executionError := backtestService.indicatorScriptProxy.ExecuteForEachCandle(
			executionContext,
			tradingStrategyBacktestDomain.SourceScript(sourceIndex),
			tradingStrategyBacktestDomain.ResultType(),
			inputKCandles,
			tradingStrategyBacktestDomain.SourceParameters(sourceIndex))
		// One source failing ends the whole replay. Half a replay is not a shorter
		// replay: the conditions would be answered against signals that are simply
		// absent, and would quietly come out false.
		if executionError != nil {
			return dto.BacktestResultDto{}, executionError
		}

		signalsBySource = append(signalsBySource, signalsOf(perCandleIndicatorValues))
	}

	return tradingStrategyBacktestDomain.ReplayOver(inputKCandles, signalsBySource), nil
}

// signalsOf reads a script's per-candle results as per-candle opinions, which is the
// one step both kinds of replay do identically.
func signalsOf(perCandleIndicatorValues []map[string]vo.IndicatorValueVo) []domains.SignalDomain {
	signals := make([]domains.SignalDomain, 0, len(perCandleIndicatorValues))
	for _, indicatorValues := range perCandleIndicatorValues {
		signals = append(signals, domains.NewSignalDomain(indicatorValues))
	}

	return signals
}
