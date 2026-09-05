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
	// signals holds exactly one opinion per candle, settled at construction. Pairing
	// them there rather than while walking is what lets the walk index both lists
	// without ever asking whether the second one is long enough.
	signals []SignalDomain
}

// NewBacktestSimulationDomain pairs each candle with the indicator result the script
// produced while standing on it: the nth result belongs to the nth candle. A candle
// with no result of its own is read as flat, which is the same thing a script that
// never named a signal says.
func NewBacktestSimulationDomain(
	initialCapital decimal.Decimal,
	positionSizing PositionSizingDomain,
	inputKCandles []vo.KCandleVo,
	perCandleIndicatorValues []map[string]vo.IndicatorValueVo,
) BacktestSimulationDomain {
	signals := make([]SignalDomain, 0, len(inputKCandles))
	for candleIndex := range inputKCandles {
		if candleIndex >= len(perCandleIndicatorValues) {
			signals = append(signals, NewSignalDomain(nil))
			continue
		}

		signals = append(signals, NewSignalDomain(perCandleIndicatorValues[candleIndex]))
	}

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
