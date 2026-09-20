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
	initialCapital   decimal.Decimal
	positionSizing   PositionSizingDomain
	tradingMode      TradingModeDomain
	exitLevels       BacktestExitLevelsDomain
	leverage         BacktestLeverageDomain
	transactionCosts BacktestTransactionCostsDomain
	inputKCandles    []vo.KCandleVo
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
	tradingMode TradingModeDomain,
	exitLevels BacktestExitLevelsDomain,
	leverage BacktestLeverageDomain,
	transactionCosts BacktestTransactionCostsDomain,
	inputKCandles []vo.KCandleVo,
	signals []SignalDomain,
) BacktestSimulationDomain {
	return BacktestSimulationDomain{
		initialCapital:   initialCapital,
		positionSizing:   positionSizing,
		tradingMode:      tradingMode,
		exitLevels:       exitLevels,
		leverage:         leverage,
		transactionCosts: transactionCosts,
		inputKCandles:    inputKCandles,
		signals:          signals,
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
		backtestSimulationDomain.positionSizing,
		backtestSimulationDomain.tradingMode,
		backtestSimulationDomain.exitLevels,
		backtestSimulationDomain.leverage,
		backtestSimulationDomain.transactionCosts)
	equityCurve := NewBacktestEquityCurveDomain(backtestSimulationDomain.initialCapital)

	for candleIndex, inputKCandle := range backtestSimulationDomain.inputKCandles {
		// Everything fills at this candle's close: the candle that spoke is the candle
		// that traded. This is the only place a fill price is decided, so filling at
		// the next candle's open later is one expression rather than a hunt.
		fillPrice := decimal.NewFromFloat(inputKCandle.Close)
		candleTime := time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC()

		// The exit levels are asked first, and that ordering is a rule rather than a
		// preference. A position is opened on the line below, so the earliest candle
		// that can stop it out is the next one round — which is right, because the
		// entry filled at this candle's close and its high and low had already
		// happened by then. Written as a check instead, that rule would be a
		// comparison of times, and a comparison of times is something a timezone or
		// two bars opening in the same second can get wrong silently.
		account.ApplyExitLevels(inputKCandle, candleTime)
		account.Apply(backtestSimulationDomain.signals[candleIndex], candleTime, fillPrice)
		equityCurve.Record(candleTime, account.EquityAt(fillPrice))
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
		// Zero for a replay that borrowed nothing, which is every replay made before
		// there was anything to borrow.
		LiquidationExitCount: account.ExitCountFor(vo.TradeExitReasonLiquidation),
		// Zero for a replay given no rates, which is every replay made before there
		// were rates to give.
		TotalTransactionCost: account.TotalTransactionCost(),
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
