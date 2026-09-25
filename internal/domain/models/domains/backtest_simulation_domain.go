package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestSimulationDomain replays a strategy script candle by candle as pure arithmetic (no storage, clock or interpreter), so trading rules are table-testable.
type BacktestSimulationDomain struct {
	initialCapital decimal.Decimal
	positionTerms  BacktestPositionTermsDomain
	fillTiming     BacktestFillTimingDomain
	inputKCandles  []vo.KCandleVo
	// signals holds exactly one opinion per candle; the script runner guarantees equal length.
	signals []SignalDomain
}

// NewBacktestSimulationDomain expects signals[n] to be the opinion produced on inputKCandles[n].
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

// ToDto produces the summary, trades and equity curve in a single walk so they cannot disagree.
func (backtestSimulationDomain BacktestSimulationDomain) ToDto() dto.BacktestResultDto {
	account := NewBacktestAccountDomain(
		backtestSimulationDomain.initialCapital,
		backtestSimulationDomain.positionTerms)
	equityCurve := NewBacktestEquityCurveDomain(backtestSimulationDomain.initialCapital)

	for candleIndex, inputKCandle := range backtestSimulationDomain.inputKCandles {
		candleTime := time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC()
		closePrice := decimal.NewFromFloat(inputKCandle.Close)

		// Next-open fills apply the previous bar's signal at this bar's open before checking exits, since this bar's high and low come after that fill; the last bar's signal is never filled.
		if backtestSimulationDomain.fillTiming.FillsAtNextOpen() {
			if candleIndex > 0 {
				previousSignal := backtestSimulationDomain.signals[candleIndex-1]
				account.Apply(previousSignal, candleTime, decimal.NewFromFloat(inputKCandle.Open))
			}
			account.ApplyExitLevels(inputKCandle, candleTime)
			equityCurve.Record(candleTime, account.EquityAt(closePrice))

			continue
		}

		// Close fills check exits before the signal, so a position opened at this close is first examined on the next candle.
		account.ApplyExitLevels(inputKCandle, candleTime)
		account.Apply(backtestSimulationDomain.signals[candleIndex], candleTime, closePrice)
		equityCurve.Record(candleTime, account.EquityAt(closePrice))
	}

	backtestSummaryDto := dto.BacktestSummaryDto{
		InitialCapital:             backtestSimulationDomain.initialCapital,
		FinalEquity:                equityCurve.FinalEquity(),
		TotalReturnRate:            equityCurve.TotalReturnRate(),
		MaximumDrawdown:            equityCurve.MaximumDrawdown(),
		PositionOpenCount:          account.PositionOpenCount(),
		StopLossExitCount:          account.ExitCountFor(vo.TradeExitReasonStopLoss),
		TakeProfitExitCount:        account.ExitCountFor(vo.TradeExitReasonTakeProfit),
		TotalTransactionCost:       account.TotalTransactionCost(),
		BacktestTradeStatisticsDto: account.TradeStatisticsDto(),
	}
	// WinRate stays absent when nothing closed, so "no trades" is not reported as "every trade lost".
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
