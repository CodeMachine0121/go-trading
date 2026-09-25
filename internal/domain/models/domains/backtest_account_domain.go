package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestAccountDomain holds a replay's free cash, at most one open position (by shape, not by check) and the finished round trips.
type BacktestAccountDomain struct {
	positionTerms     BacktestPositionTermsDomain
	availableCash     decimal.Decimal
	openPosition      BacktestPositionDomain
	hasOpenPosition   bool
	positionOpenCount int
	closedTrades      []vo.ClosedTradeVo
}

func NewBacktestAccountDomain(
	initialCapital decimal.Decimal,
	positionTerms BacktestPositionTermsDomain,
) *BacktestAccountDomain {
	return &BacktestAccountDomain{
		positionTerms: positionTerms,
		availableCash: initialCapital,
		closedTrades:  make([]vo.ClosedTradeVo, 0),
	}
}

// ApplyExitLevels closes the open position if this candle reached its stop or target, and is a no-op without distances.
// It runs before the candle's signal, so an entry candle cannot stop itself out and a stopped-out candle still hears its signal.
func (backtestAccountDomain *BacktestAccountDomain) ApplyExitLevels(
	kCandle vo.KCandleVo, candleTime time.Time,
) {
	if !backtestAccountDomain.hasOpenPosition {
		return
	}

	closedTrade, isExited := backtestAccountDomain.openPosition.ExitOn(kCandle, candleTime)
	if !isExited {
		return
	}

	backtestAccountDomain.settleOpenPosition(closedTrade)
}

// Apply carries out one candle's target position (see SignalDomain.TargetPosition) at the fill price; asking for what is already held does nothing.
func (backtestAccountDomain *BacktestAccountDomain) Apply(
	signal SignalDomain, candleTime time.Time, fillPrice decimal.Decimal,
) {
	targetPosition := signal.TargetPosition()
	// An unchanged target must leave an open position alone, whereas a flat target closes it.
	if targetPosition == vo.TargetPositionUnchanged {
		return
	}

	wantsPosition := targetPosition.WantsPosition()

	if backtestAccountDomain.hasOpenPosition {
		if wantsPosition {
			return
		}

		backtestAccountDomain.settleOpenPosition(
			backtestAccountDomain.openPosition.ClosedAt(
				candleTime, fillPrice, vo.TradeExitReasonSignal))
	}

	if !wantsPosition {
		return
	}

	// An unaffordable opening is silently skipped; sizing, costs and exit levels all come from the position terms.
	openedPosition, isOpened := backtestAccountDomain.positionTerms.OpenFor(
		candleTime, fillPrice, backtestAccountDomain.availableCash)
	if !isOpened {
		return
	}

	// Stake and entry cost are withdrawn together; the terms already guaranteed both fit.
	backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.
		Sub(openedPosition.Stake()).Sub(openedPosition.EntryCost())
	backtestAccountDomain.openPosition = openedPosition
	backtestAccountDomain.hasOpenPosition = true
	backtestAccountDomain.positionOpenCount++
}

// settleOpenPosition is shared by both exit paths so the cash return can never be forgotten in one of them.
func (backtestAccountDomain *BacktestAccountDomain) settleOpenPosition(
	closedTrade vo.ClosedTradeVo,
) {
	backtestAccountDomain.closedTrades = append(
		backtestAccountDomain.closedTrades, closedTrade)
	backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.
		Add(backtestAccountDomain.openPosition.CashReturnedFor(closedTrade))
	backtestAccountDomain.hasOpenPosition = false
}

// EquityAt values any open position as if closed at price.
func (backtestAccountDomain *BacktestAccountDomain) EquityAt(
	price decimal.Decimal,
) decimal.Decimal {
	if !backtestAccountDomain.hasOpenPosition {
		return backtestAccountDomain.availableCash
	}

	return backtestAccountDomain.availableCash.Add(
		backtestAccountDomain.openPosition.ValueAt(price))
}

// ClosedTradeDtos returns finished round trips earliest first, excluding any open position.
func (backtestAccountDomain *BacktestAccountDomain) ClosedTradeDtos() []dto.ClosedTradeDto {
	closedTradeDtos := make([]dto.ClosedTradeDto, 0, len(backtestAccountDomain.closedTrades))
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		closedTradeDtos = append(closedTradeDtos, closedTrade.ToDto())
	}

	return closedTradeDtos
}

// ExitCountFor is derived from the trade list rather than a separate counter, so it can never disagree with it.
func (backtestAccountDomain *BacktestAccountDomain) ExitCountFor(
	exitReason vo.TradeExitReasonVo,
) int {
	exitCount := 0
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		if closedTrade.ExitReason == exitReason {
			exitCount++
		}
	}

	return exitCount
}

// TotalTransactionCost sums both charges on finished trades plus the already-paid entry charge of an open position, derived from the trade list.
func (backtestAccountDomain *BacktestAccountDomain) TotalTransactionCost() decimal.Decimal {
	totalTransactionCost := decimal.Zero
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		totalTransactionCost = totalTransactionCost.
			Add(closedTrade.EntryCost).Add(closedTrade.ExitCost)
	}

	if backtestAccountDomain.hasOpenPosition {
		totalTransactionCost = totalTransactionCost.Add(
			backtestAccountDomain.openPosition.EntryCost())
	}

	return totalTransactionCost
}

// PositionOpenCount excludes openings skipped for lack of cash.
func (backtestAccountDomain *BacktestAccountDomain) PositionOpenCount() int {
	return backtestAccountDomain.positionOpenCount
}

// WinRate returns false when no trade has finished, so "nothing finished" is distinguishable from "every trade lost".
func (backtestAccountDomain *BacktestAccountDomain) WinRate() (float64, bool) {
	if len(backtestAccountDomain.closedTrades) == 0 {
		return 0, false
	}

	winCount := 0
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		if closedTrade.IsWin() {
			winCount++
		}
	}

	return float64(winCount) / float64(len(backtestAccountDomain.closedTrades)), true
}

// TradeStatisticsDto summarises finished round trips only.
func (backtestAccountDomain *BacktestAccountDomain) TradeStatisticsDto() dto.BacktestTradeStatisticsDto {
	outcomes := make([]vo.TradeOutcomeVo, 0, len(backtestAccountDomain.closedTrades))
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		outcomes = append(outcomes, closedTrade.ToOutcomeVo())
	}

	return NewBacktestTradeStatisticsDomain(outcomes).ToDto()
}
