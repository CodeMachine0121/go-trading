package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyBacktestDomain is one spot replay of a trading strategy.
//
// Everything about the replay itself — which stretch, how much capital, how much each
// opening stakes, which candles count as finished, how the account is walked — is
// delegated to BacktestDomain untouched. Everything about where each candle's signal
// comes from — the sources, their shared coarseness, the two condition trees — is
// TradingStrategyReplaySourcesDomain's, shared with the contract replay so the two
// reach the same signal on the same candle. What is left here is only what makes it a
// spot replay: the trading strategy and every one of its sources eat K candles.
type TradingStrategyBacktestDomain struct {
	backtest BacktestDomain
	sources  TradingStrategyReplaySourcesDomain
}

// NewTradingStrategyBacktestDomain validates the request against every replay rule. A
// trading strategy written for contracts is refused before anything else: every other
// rule would be answered about candles it was never written to read.
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

	// The coarseness is the trading strategy's answer, not the caller's, which is the
	// only thing this has to supply — every other condition of the replay travels
	// with the request itself.
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

// ReplayOver turns each candle's several opinions into one and hands the result to the
// ordinary replay. The one figure only a trading strategy produces — how many candles
// its trees conflicted on — is added to the report card afterwards.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) ReplayOver(
	inputKCandles []vo.KCandleVo, signalsBySource [][]SignalDomain,
) dto.BacktestResultDto {
	verdicts, conflictedCandleCount := tradingStrategyBacktestDomain.sources.Combine(
		len(inputKCandles), signalsBySource)

	backtestResultDto := tradingStrategyBacktestDomain.backtest.ReplayOver(inputKCandles, verdicts)
	backtestResultDto.Summary.ConflictedCandleCount = conflictedCandleCount

	return backtestResultDto
}
