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
	backtest, backtestError := NewBacktestDomain(
		requestDto.ToBacktestRequestDto(sources.SharedInterval()), maxCandleCount, now)
	if backtestError != nil {
		return TradingStrategyBacktestDomain{}, backtestError
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
