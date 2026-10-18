package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyBacktestDomain delegates the replay to BacktestDomain and signal
// combination to TradingStrategyReplaySourcesDomain, adding only the spot-only checks.
type TradingStrategyBacktestDomain struct {
	backtest BacktestDomain
	sources  TradingStrategyReplaySourcesDomain
}

// NewTradingStrategyBacktestDomain refuses contract trading strategies before any other rule.
func NewTradingStrategyBacktestDomain(
	requestDto dto.TradingStrategyBacktestRequestDto, maxCandleCount int, now time.Time,
) (TradingStrategyBacktestDomain, error) {
	tradingStrategyKind, kindError := NewMarketDataKindDomain(requestDto.TradingStrategyMarketDataKind)
	if kindError != nil {
		return TradingStrategyBacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, kindError)
	}
	if tradingStrategyKind.value != vo.MarketDataKindKCandle {
		return TradingStrategyBacktestDomain{}, fmt.Errorf(
			"%w: 這份交易策略吃的是%s，不能拿來做現貨重演——請改用合約重演",
			ErrBacktestValidation, tradingStrategyKind.label())
	}

	sources, sourcesError := NewTradingStrategyReplaySourcesDomain(
		requestDto.SignalSources, requestDto.BuyCondition, requestDto.SellCondition, tradingStrategyKind)
	if sourcesError != nil {
		return TradingStrategyBacktestDomain{}, sourcesError
	}

	// The interval comes from the trading strategy's sources, not the caller.
	pace := sources.Pace()
	backtest, backtestError := NewBacktestDomain(
		requestDto.ToBacktestRequestDto(string(pace.Interval().Value())), maxCandleCount, now)
	if backtestError != nil {
		return TradingStrategyBacktestDomain{}, backtestError
	}

	backtest = backtest.warmedUpFor(pace.WarmupBarCount())
	if pace.MixesIntervals() {
		if warmupError := backtest.refusingWarmupBeyond(maxCandleCount); warmupError != nil {
			return TradingStrategyBacktestDomain{}, warmupError
		}
		backtest = backtest.readingAlignedTo(pace.CoarsestInterval())
	}

	return TradingStrategyBacktestDomain{backtest: backtest, sources: sources}, nil
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) Symbol() string {
	return tradingStrategyBacktestDomain.backtest.Symbol()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) KCandleQuery() KCandleQueryDomain {
	return tradingStrategyBacktestDomain.backtest.KCandleQuery()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceCandleLimit() int {
	return tradingStrategyBacktestDomain.backtest.SourceCandleLimit()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SelectInputCandles(
	kCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	return tradingStrategyBacktestDomain.backtest.SelectInputCandles(kCandles)
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceIsAtPace(index int) bool {
	return tradingStrategyBacktestDomain.sources.SourceIsAtPace(index)
}

// SourceInputCandles is what a source coarser than the pace reads: its own finished bars from the same candles.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceInputCandles(
	index int, kCandles []entities.KCandle,
) []vo.KCandleVo {
	return tradingStrategyBacktestDomain.backtest.FinishedCandlesAt(
		tradingStrategyBacktestDomain.sources.SourceInterval(index), kCandles)
}

// SignalsAtPace lets a coarser source speak on each pace candle with its last finished bar only.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SignalsAtPace(
	index int, sourceKCandles []vo.KCandleVo, sourceSignals []SignalDomain, paceKCandles []vo.KCandleVo,
) []SignalDomain {
	return tradingStrategyBacktestDomain.sources.Pace().SignalsAtPace(
		tradingStrategyBacktestDomain.sources.SourceInterval(index),
		tradingStrategyBacktestDomain.backtest.openTimesOf(sourceKCandles),
		sourceSignals,
		tradingStrategyBacktestDomain.backtest.openTimesOf(paceKCandles))
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) ResultType() IndicatorResultTypeDomain {
	return tradingStrategyBacktestDomain.backtest.ResultType()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceCount() int {
	return tradingStrategyBacktestDomain.sources.SourceCount()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceScript(index int) string {
	return tradingStrategyBacktestDomain.sources.SourceScript(index)
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceParameters(
	index int,
) StrategyScriptParametersDomain {
	return tradingStrategyBacktestDomain.sources.SourceParameters(index)
}

// ReplayOver combines each candle's source signals, replays them, then adds the
// conflicted-candle count to the summary.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) ReplayOver(
	inputKCandles []vo.KCandleVo, signalsBySource [][]SignalDomain,
) dto.BacktestResultDto {
	verdicts, conflictedFlags := tradingStrategyBacktestDomain.sources.Combine(
		len(inputKCandles), signalsBySource)

	return tradingStrategyBacktestDomain.backtest.ReplayOver(inputKCandles, verdicts, conflictedFlags)
}
