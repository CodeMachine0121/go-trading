package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// BacktestService is the application layer's only entry point for replaying a
// strategy over a stretch of market that has already happened.
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
// the one it stands on — so a strategy that looks back further than it has candles
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

	return backtestDomain.ReplayOver(inputKCandles, perCandleIndicatorValues), nil
}
