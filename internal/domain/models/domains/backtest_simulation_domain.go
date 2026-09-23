package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestSimulationDomain replays one strategy script candle by candle.
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
	positionTerms  BacktestPositionTermsDomain
	fillTiming     BacktestFillTimingDomain
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
	positionTerms BacktestPositionTermsDomain,
	fillTiming BacktestFillTimingDomain,
	inputKCandles []vo.KCandleVo,
	signals []SignalDomain,
) BacktestSimulationDomain {
	return BacktestSimulationDomain{
		initialCapital: initialCapital,
		positionTerms:  positionTerms,
		fillTiming:     fillTiming,
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
		backtestSimulationDomain.initialCapital,
		backtestSimulationDomain.positionTerms)
	equityCurve := NewBacktestEquityCurveDomain(backtestSimulationDomain.initialCapital)

	for candleIndex, inputKCandle := range backtestSimulationDomain.inputKCandles {
		candleTime := time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC()
		closePrice := decimal.NewFromFloat(inputKCandle.Close)

		// Filling at the next open, the previous bar's opinion is carried out first, at
		// this bar's open — and only then are the exit levels asked, because this bar's
		// high and low happen after that fill, even for the position it just opened.
		// The last bar's opinion has no next bar and is never filled.
		if backtestSimulationDomain.fillTiming.FillsAtNextOpen() {
			if candleIndex > 0 {
				previousSignal := backtestSimulationDomain.signals[candleIndex-1]
				account.Apply(previousSignal, candleTime, decimal.NewFromFloat(inputKCandle.Open))
			}
			account.ApplyExitLevels(inputKCandle, candleTime)
			equityCurve.Record(candleTime, account.EquityAt(closePrice))

			continue
		}

		// Filling at the close, the candle that spoke is the candle that traded. The
		// exit levels are asked first, and that ordering is a rule rather than a
		// preference: a position opened on the line below is first examined on the next
		// candle round, which is right, because the entry filled at this candle's close
		// and its high and low had already happened by then.
		account.ApplyExitLevels(inputKCandle, candleTime)
		account.Apply(backtestSimulationDomain.signals[candleIndex], candleTime, closePrice)
		equityCurve.Record(candleTime, account.EquityAt(closePrice))
	}

	backtestSummaryDto := dto.BacktestSummaryDto{
		InitialCapital:    backtestSimulationDomain.initialCapital,
		FinalEquity:       equityCurve.FinalEquity(),
		TotalReturnRate:   equityCurve.TotalReturnRate(),
		MaximumDrawdown:   equityCurve.MaximumDrawdown(),
		PositionOpenCount: account.PositionOpenCount(),
		// Both counts are zero for a replay given no distances, which is every
		// replay made before there were distances to give.
		StopLossExitCount:   account.ExitCountFor(vo.TradeExitReasonStopLoss),
		TakeProfitExitCount: account.ExitCountFor(vo.TradeExitReasonTakeProfit),
		// Zero for a replay given no rates, which is every replay made before there
		// were rates to give.
		TotalTransactionCost: account.TotalTransactionCost(),
		// What the finished round trips say about a short-term strategy.
		BacktestTradeStatisticsDto: account.TradeStatisticsDto(),
	}
	// The win rate stays absent when nothing was ever closed, which is what keeps "no
	// trades" from being reported as "every trade lost".
	if winRate, isApplicable := account.WinRate(); isApplicable {
		backtestSummaryDto.WinRate = &winRate
	}

	return dto.BacktestResultDto{
		FillTiming:      string(backtestSimulationDomain.fillTiming.Value()),
		UsedCandleCount: len(backtestSimulationDomain.inputKCandles),
		Summary:         backtestSummaryDto,
		ClosedTrades:    account.ClosedTradeDtos(),
		EquityCurve:     equityCurve.PointDtos(),
	}
}
