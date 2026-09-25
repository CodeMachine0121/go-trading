package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestPositionDomain is a replay's single long spot position; unit count and exit prices are fixed at entry so valuations never drift from re-division.
type BacktestPositionDomain struct {
	entryTime  time.Time
	entryPrice decimal.Decimal
	// stake is both the cash withdrawn and the amount at risk, since spot borrows nothing.
	stake      decimal.Decimal
	unitCount  decimal.Decimal
	exitPrices vo.ExitPricesVo
	// transactionCosts are carried so the exit is charged at the rates the position was opened under.
	transactionCosts BacktestTransactionCostsDomain
	// entryCost is fixed at entry and is the only record that it was already paid.
	entryCost decimal.Decimal
}

// newBacktestPositionDomain is unexported so BacktestPositionTermsDomain.OpenFor is the only way to open a position; a non-positive price or stake is refused instead of dividing by it.
func newBacktestPositionDomain(
	entryTime time.Time,
	entryPrice decimal.Decimal,
	stake decimal.Decimal,
	exitLevels BacktestExitLevelsDomain,
	transactionCosts BacktestTransactionCostsDomain,
) (BacktestPositionDomain, bool) {
	if !entryPrice.IsPositive() || !stake.IsPositive() {
		return BacktestPositionDomain{}, false
	}

	return BacktestPositionDomain{
		entryTime:        entryTime.UTC(),
		entryPrice:       entryPrice,
		stake:            stake,
		unitCount:        stake.Div(entryPrice),
		exitPrices:       exitLevels.PricesFrom(entryPrice),
		transactionCosts: transactionCosts,
		entryCost:        transactionCosts.EntryCostFor(stake),
	}, true
}

func (backtestPositionDomain BacktestPositionDomain) Stake() decimal.Decimal {
	return backtestPositionDomain.stake
}

func (backtestPositionDomain BacktestPositionDomain) EntryCost() decimal.Decimal {
	return backtestPositionDomain.entryCost
}

func (backtestPositionDomain BacktestPositionDomain) ProfitAt(
	price decimal.Decimal,
) decimal.Decimal {
	return backtestPositionDomain.unitCount.Mul(
		price.Sub(backtestPositionDomain.entryPrice))
}

// ValueAt is the stake plus profit at price, deliberately gross of exit cost, which is only deducted when the position is actually closed.
// A replay ending with an open position therefore reports final equity one exit charge too high.
func (backtestPositionDomain BacktestPositionDomain) ValueAt(
	price decimal.Decimal,
) decimal.Decimal {
	return backtestPositionDomain.stake.Add(backtestPositionDomain.ProfitAt(price))
}

// ExitOn checks the candle's low and high (not its close) against the exit prices, testing the stop first so bars touching both never flatter the strategy.
// The entry candle is never checked because the walk applies exits before the candle's signal.
func (backtestPositionDomain BacktestPositionDomain) ExitOn(
	kCandle vo.KCandleVo, exitTime time.Time,
) (vo.ClosedTradeVo, bool) {
	candleHigh := decimal.NewFromFloat(kCandle.High)
	candleLow := decimal.NewFromFloat(kCandle.Low)

	if backtestPositionDomain.exitPrices.HasStopLoss &&
		candleLow.LessThanOrEqual(backtestPositionDomain.exitPrices.StopLossPrice) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.StopLossPrice,
			vo.TradeExitReasonStopLoss), true
	}

	if backtestPositionDomain.exitPrices.HasTakeProfit &&
		candleHigh.GreaterThanOrEqual(backtestPositionDomain.exitPrices.TakeProfitPrice) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.TakeProfitPrice,
			vo.TradeExitReasonTakeProfit), true
	}

	return vo.ClosedTradeVo{}, false
}

// ClosedAt records the exit reason explicitly and reports profit net of both charges, so the win rate cannot flatter a strategy whose edge is eaten by costs.
func (backtestPositionDomain BacktestPositionDomain) ClosedAt(
	exitTime time.Time, exitPrice decimal.Decimal, exitReason vo.TradeExitReasonVo,
) vo.ClosedTradeVo {
	exitCost := backtestPositionDomain.transactionCosts.ExitCostFor(
		backtestPositionDomain.unitCount.Mul(exitPrice))

	return vo.ClosedTradeVo{
		Direction:  vo.PositionDirectionLong,
		EntryTime:  backtestPositionDomain.entryTime,
		EntryPrice: backtestPositionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Stake:      backtestPositionDomain.stake,
		EntryCost:  backtestPositionDomain.entryCost,
		ExitCost:   exitCost,
		Profit: backtestPositionDomain.ProfitAt(exitPrice).
			Sub(backtestPositionDomain.entryCost).Sub(exitCost),
		ExitReason: exitReason,
	}
}

// CashReturnedFor is the position's value at the exit price less the exit charge; no floor is needed since a spot position cannot go below zero.
func (backtestPositionDomain BacktestPositionDomain) CashReturnedFor(
	closedTrade vo.ClosedTradeVo,
) decimal.Decimal {
	return backtestPositionDomain.ValueAt(closedTrade.ExitPrice).Sub(closedTrade.ExitCost)
}
