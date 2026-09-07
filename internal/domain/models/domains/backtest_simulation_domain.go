package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestSimulationDomain replays one strategy candle by candle.
//
// It is pure arithmetic: candles in, opinions in, a report card out. It reads no
// storage, no clock and no interpreter, which is what lets every rule the requirements
// state be pinned by a table of numbers rather than by a running system.
//
// It is deliberately not part of BacktestDomain. That one changes when the rules about
// what may be replayed change; this one changes when the rules about trading change —
// fees, stops, filling at the next candle's open. Two reasons to change, two models.
type BacktestSimulationDomain struct {
	initialCapital decimal.Decimal
	positionSizing PositionSizingDomain
	inputKCandles  []vo.KCandleVo
	// signals holds exactly one opinion per candle: the nth belongs to the nth
	// candle. The script runner produces one signal per candle or fails the whole
	// replay, so by here the two lists are always the same length.
	signals []SignalDomain
}

// NewBacktestSimulationDomain takes the candles and the one signal read off each of
// them, already paired: signals[n] is the opinion the script produced while standing
// on inputKCandles[n]. A replay whose script did not produce a signal on some candle
// never reaches here — that is a script failure, caught where the script is run.
func NewBacktestSimulationDomain(
	initialCapital decimal.Decimal,
	positionSizing PositionSizingDomain,
	inputKCandles []vo.KCandleVo,
	signals []SignalDomain,
) BacktestSimulationDomain {
	return BacktestSimulationDomain{
		initialCapital: initialCapital,
		positionSizing: positionSizing,
		inputKCandles:  inputKCandles,
		signals:        signals,
	}
}

// ToDto walks the candles once and hands back everything the replay produced.
//
// One walk rather than three: the report card, the finished trades and the equity
// curve are three views of the same history, and producing them separately would leave
// three chances for them to disagree about what happened.
func (backtestSimulationDomain BacktestSimulationDomain) ToDto() dto.BacktestResultDto {
	account := NewBacktestAccountDomain(
		backtestSimulationDomain.initialCapital, backtestSimulationDomain.positionSizing)
	equityCurve := NewBacktestEquityCurveDomain(backtestSimulationDomain.initialCapital)

	for candleIndex, inputKCandle := range backtestSimulationDomain.inputKCandles {
		// Everything fills at this candle's close: the candle that spoke is the candle
		// that traded. This is the only place a fill price is decided, so filling at
		// the next candle's open later is one expression rather than a hunt.
		fillPrice := decimal.NewFromFloat(inputKCandle.Close)
		candleTime := time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC()

		account.Apply(backtestSimulationDomain.signals[candleIndex], candleTime, fillPrice)
		equityCurve.Record(candleTime, account.EquityAt(fillPrice))
	}

	backtestSummaryDto := dto.BacktestSummaryDto{
		InitialCapital:    backtestSimulationDomain.initialCapital,
		FinalEquity:       equityCurve.FinalEquity(),
		TotalReturnRate:   equityCurve.TotalReturnRate(),
		MaximumDrawdown:   equityCurve.MaximumDrawdown(),
		PositionOpenCount: account.PositionOpenCount(),
	}
	// The win rate stays absent when nothing was ever closed, which is what keeps "no
	// trades" from being reported as "every trade lost".
	if winRate, isApplicable := account.WinRate(); isApplicable {
		backtestSummaryDto.WinRate = &winRate
	}

	return dto.BacktestResultDto{
		UsedCandleCount: len(backtestSimulationDomain.inputKCandles),
		Summary:         backtestSummaryDto,
		ClosedTrades:    account.ClosedTradeDtos(),
		EquityCurve:     equityCurve.PointDtos(),
	}
}
