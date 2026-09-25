package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

var oneWhole = decimal.NewFromInt(1)

// ContractBacktestPositionDomain is a replay's single isolated-margin contract position; funding moves its margin, so its liquidation price shifts over time unlike its stop.
type ContractBacktestPositionDomain struct {
	direction  vo.PositionDirectionVo
	entryTime  time.Time
	entryPrice decimal.Decimal
	leverage   decimal.Decimal
	quantity   decimal.Decimal
	// openingMargin is what was withdrawn to open; margin is the current balance after funding.
	openingMargin    decimal.Decimal
	margin           decimal.Decimal
	fundingFeePaid   decimal.Decimal
	exitPrices       vo.ExitPricesVo
	maintenanceTier  vo.ContractMaintenanceMarginTierVo
	transactionCosts BacktestTransactionCostsDomain
	slippage         BacktestSlippageDomain
	entryCost        decimal.Decimal
}

func (positionDomain ContractBacktestPositionDomain) Direction() vo.PositionDirectionVo {
	return positionDomain.direction
}

func (positionDomain ContractBacktestPositionDomain) Quantity() decimal.Decimal {
	return positionDomain.quantity
}

func (positionDomain ContractBacktestPositionDomain) EntryPrice() decimal.Decimal {
	return positionDomain.entryPrice
}

// ExitPrices are already rounded to the venue's ticks.
func (positionDomain ContractBacktestPositionDomain) ExitPrices() vo.ExitPricesVo {
	return positionDomain.exitPrices
}

func (positionDomain ContractBacktestPositionDomain) OpeningMargin() decimal.Decimal {
	return positionDomain.openingMargin
}

func (positionDomain ContractBacktestPositionDomain) EntryCost() decimal.Decimal {
	return positionDomain.entryCost
}

// FundingFeePaid is net funding paid; negative means the position was paid.
func (positionDomain ContractBacktestPositionDomain) FundingFeePaid() decimal.Decimal {
	return positionDomain.fundingFeePaid
}

// LiquidationPrice solves equity = maintenance margin (q × mark × r − c): long (qE − M − c) ÷ q(1 − r), short (qE + M + c) ÷ q(1 + r).
// A long whose result is not positive cannot be liquidated.
func (positionDomain ContractBacktestPositionDomain) LiquidationPrice() decimal.Decimal {
	quantityAtEntry := positionDomain.quantity.Mul(positionDomain.entryPrice)
	cushion := positionDomain.margin.Add(positionDomain.maintenanceTier.MaintenanceAmount)
	rate := positionDomain.maintenanceTier.MaintenanceMarginRate

	if positionDomain.direction == vo.PositionDirectionShort {
		return quantityAtEntry.Add(cushion).Div(positionDomain.quantity.Mul(oneWhole.Add(rate)))
	}

	return quantityAtEntry.Sub(cushion).Div(positionDomain.quantity.Mul(oneWhole.Sub(rate)))
}

// SettleFunding charges quantity × mark × rate, paid by longs and received by shorts when the rate is positive, adjusting the margin.
func (positionDomain *ContractBacktestPositionDomain) SettleFunding(
	fundingRate decimal.Decimal, markPrice decimal.Decimal,
) {
	fundingFee := positionDomain.quantity.Mul(markPrice).Mul(fundingRate)
	if positionDomain.direction == vo.PositionDirectionShort {
		fundingFee = fundingFee.Neg()
	}

	positionDomain.fundingFeePaid = positionDomain.fundingFeePaid.Add(fundingFee)
	positionDomain.margin = positionDomain.margin.Sub(fundingFee)
}

func (positionDomain ContractBacktestPositionDomain) ProfitAt(price decimal.Decimal) decimal.Decimal {
	priceMove := price.Sub(positionDomain.entryPrice)
	if positionDomain.direction == vo.PositionDirectionShort {
		priceMove = priceMove.Neg()
	}

	return positionDomain.quantity.Mul(priceMove)
}

// ValueAt never goes below zero, since an isolated position loses at most its margin.
func (positionDomain ContractBacktestPositionDomain) ValueAt(price decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Zero, positionDomain.margin.Add(positionDomain.ProfitAt(price)))
}

// ExitOn settles the open first: a mark open past liquidation liquidates, otherwise a traded open past the stop fills there.
// Within the bar the adverse side still comes first, taking whichever of the stop (traded high/low) or liquidation (mark price) is nearer the entry, then the take profit, so ambiguous bars never flatter the strategy.
func (positionDomain ContractBacktestPositionDomain) ExitOn(
	bucket dto.KCandleContractDto, exitTime time.Time,
) (vo.ContractClosedTradeVo, bool) {
	isShort := positionDomain.direction == vo.PositionDirectionShort
	liquidationPrice := positionDomain.LiquidationPrice()
	hasStopLoss := positionDomain.exitPrices.HasStopLoss
	stopLossPrice := positionDomain.exitPrices.StopLossPrice

	liquidatedAtOpen := (!isShort && liquidationPrice.IsPositive() &&
		bucket.MarkOpen.LessThanOrEqual(liquidationPrice)) ||
		(isShort && bucket.MarkOpen.GreaterThanOrEqual(liquidationPrice))
	if liquidatedAtOpen {
		return positionDomain.ClosedAt(exitTime, liquidationPrice, vo.TradeExitReasonLiquidation), true
	}

	stoppedAtOpen := hasStopLoss &&
		((!isShort && bucket.Open.LessThanOrEqual(stopLossPrice)) ||
			(isShort && bucket.Open.GreaterThanOrEqual(stopLossPrice)))
	if stoppedAtOpen {
		return positionDomain.ClosedAt(exitTime,
			positionDomain.exitFillFor(bucket.Open), vo.TradeExitReasonStopLoss), true
	}

	stopReached := hasStopLoss &&
		((!isShort && bucket.Low.LessThanOrEqual(stopLossPrice)) ||
			(isShort && bucket.High.GreaterThanOrEqual(stopLossPrice)))
	liquidationReached := (!isShort && liquidationPrice.IsPositive() &&
		bucket.MarkLow.LessThanOrEqual(liquidationPrice)) ||
		(isShort && bucket.MarkHigh.GreaterThanOrEqual(liquidationPrice))
	stopIsNearer := hasStopLoss &&
		((!isShort && stopLossPrice.GreaterThanOrEqual(liquidationPrice)) ||
			(isShort && stopLossPrice.LessThanOrEqual(liquidationPrice)))

	if stopReached && (stopIsNearer || !liquidationReached) {
		return positionDomain.ClosedAt(exitTime,
			positionDomain.exitFillFor(stopLossPrice), vo.TradeExitReasonStopLoss), true
	}

	if liquidationReached {
		return positionDomain.ClosedAt(exitTime, liquidationPrice, vo.TradeExitReasonLiquidation), true
	}

	takeProfitReached := positionDomain.exitPrices.HasTakeProfit &&
		((!isShort && bucket.High.GreaterThanOrEqual(positionDomain.exitPrices.TakeProfitPrice)) ||
			(isShort && bucket.Low.LessThanOrEqual(positionDomain.exitPrices.TakeProfitPrice)))
	if takeProfitReached {
		return positionDomain.ClosedAt(exitTime,
			positionDomain.exitFillFor(positionDomain.exitPrices.TakeProfitPrice),
			vo.TradeExitReasonTakeProfit), true
	}

	return vo.ContractClosedTradeVo{}, false
}

// ClosedBySignalAt closes at the bar's close with slippage applied.
func (positionDomain ContractBacktestPositionDomain) ClosedBySignalAt(
	exitTime time.Time, closePrice decimal.Decimal,
) vo.ContractClosedTradeVo {
	return positionDomain.ClosedAt(
		exitTime, positionDomain.exitFillFor(closePrice), vo.TradeExitReasonSignal)
}

// exitFillFor applies slippage against the position for every trading exit (stop, take profit, signal).
func (positionDomain ContractBacktestPositionDomain) exitFillFor(price decimal.Decimal) decimal.Decimal {
	if positionDomain.direction == vo.PositionDirectionShort {
		return positionDomain.slippage.BuyingAt(price)
	}

	return positionDomain.slippage.SellingAt(price)
}

// ClosedAt makes a liquidation lose exactly the opening margin plus entry charge, since the liquidation fee and funding already came out of the margin.
func (positionDomain ContractBacktestPositionDomain) ClosedAt(
	exitTime time.Time, exitPrice decimal.Decimal, exitReason vo.TradeExitReasonVo,
) vo.ContractClosedTradeVo {
	closedTrade := vo.ContractClosedTradeVo{
		Direction:  positionDomain.direction,
		EntryTime:  positionDomain.entryTime,
		EntryPrice: positionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Leverage:   positionDomain.leverage,
		Quantity:   positionDomain.quantity,
		Margin:     positionDomain.openingMargin,
		EntryCost:  positionDomain.entryCost,
		ExitCost:   decimal.Zero,
		FundingFee: positionDomain.fundingFeePaid,
		ExitReason: exitReason,
	}

	if exitReason == vo.TradeExitReasonLiquidation {
		closedTrade.Profit = positionDomain.openingMargin.Add(positionDomain.entryCost).Neg()

		return closedTrade
	}

	closedTrade.ExitCost = positionDomain.transactionCosts.ExitCostFor(
		positionDomain.quantity.Mul(exitPrice))
	// Profit is cash returned minus what was put down, capped at losing the margin when the traded price overshoots the mark.
	closedTrade.Profit = positionDomain.CashReturnedFor(closedTrade).
		Sub(positionDomain.openingMargin).Sub(positionDomain.entryCost)

	return closedTrade
}

// CashReturnedFor is zero after liquidation and never negative otherwise.
func (positionDomain ContractBacktestPositionDomain) CashReturnedFor(
	closedTrade vo.ContractClosedTradeVo,
) decimal.Decimal {
	if closedTrade.ExitReason == vo.TradeExitReasonLiquidation {
		return decimal.Zero
	}

	return decimal.Max(decimal.Zero, positionDomain.margin.
		Add(positionDomain.ProfitAt(closedTrade.ExitPrice)).Sub(closedTrade.ExitCost))
}
