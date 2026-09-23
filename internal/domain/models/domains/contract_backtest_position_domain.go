package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

var oneWhole = decimal.NewFromInt(1)

// ContractBacktestPositionDomain is the one isolated-margin contract position a replay
// is holding: which way it faces, what it was entered at, the margin put down on it,
// the leverage and the units that bought, its exit prices, the maintenance margin tier
// it was opened in, and the funding it has paid or received since.
//
// **Its margin moves.** Funding is paid out of it and received into it, the way an
// isolated position's margin does — so its liquidation price is not settled once at
// entry like its stop is. Paying funding walks the liquidation price towards the entry;
// being paid walks it away.
type ContractBacktestPositionDomain struct {
	direction  vo.PositionDirectionVo
	entryTime  time.Time
	entryPrice decimal.Decimal
	leverage   decimal.Decimal
	quantity   decimal.Decimal
	// openingMargin is what was taken out of the account to open this; margin is what
	// the position holds now, after the funding it has paid and received.
	openingMargin    decimal.Decimal
	margin           decimal.Decimal
	fundingFeePaid   decimal.Decimal
	exitPrices       vo.ExitPricesVo
	maintenanceTier  vo.ContractMaintenanceMarginTierVo
	transactionCosts BacktestTransactionCostsDomain
	slippage         BacktestSlippageDomain
	entryCost        decimal.Decimal
}

// Direction is which way this position faces.
func (positionDomain ContractBacktestPositionDomain) Direction() vo.PositionDirectionVo {
	return positionDomain.direction
}

// OpeningMargin is what was taken out of the account to open this.
func (positionDomain ContractBacktestPositionDomain) OpeningMargin() decimal.Decimal {
	return positionDomain.openingMargin
}

// EntryCost is what was already paid to open this.
func (positionDomain ContractBacktestPositionDomain) EntryCost() decimal.Decimal {
	return positionDomain.entryCost
}

// FundingFeePaid is the funding this position has paid so far, net of what it has
// received; negative is money it was paid.
func (positionDomain ContractBacktestPositionDomain) FundingFeePaid() decimal.Decimal {
	return positionDomain.fundingFeePaid
}

// LiquidationPrice is the mark price at which this position's margin plus what it has
// made or lost falls to its maintenance margin.
//
// Maintenance margin is quantity × mark price × the tier's rate, less the tier's
// maintenance amount; setting the position's equity equal to it and solving for the
// price gives, for quantity q, entry E, margin M, rate r and amount c:
// long (qE − M − c) ÷ q(1 − r), short (qE + M + c) ÷ q(1 + r).
// A long whose answer is not above zero cannot be liquidated at any price.
func (positionDomain ContractBacktestPositionDomain) LiquidationPrice() decimal.Decimal {
	quantityAtEntry := positionDomain.quantity.Mul(positionDomain.entryPrice)
	cushion := positionDomain.margin.Add(positionDomain.maintenanceTier.MaintenanceAmount)
	rate := positionDomain.maintenanceTier.MaintenanceMarginRate

	if positionDomain.direction == vo.PositionDirectionShort {
		return quantityAtEntry.Add(cushion).Div(positionDomain.quantity.Mul(oneWhole.Add(rate)))
	}

	return quantityAtEntry.Sub(cushion).Div(positionDomain.quantity.Mul(oneWhole.Sub(rate)))
}

// SettleFunding pays or receives one funding settlement: quantity × mark price × rate,
// paid by a long and received by a short when the rate is positive, the other way
// round when it is negative. It moves the margin, and so the liquidation price.
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

// ProfitAt is what the price has made or lost this position had it been closed there.
func (positionDomain ContractBacktestPositionDomain) ProfitAt(price decimal.Decimal) decimal.Decimal {
	priceMove := price.Sub(positionDomain.entryPrice)
	if positionDomain.direction == vo.PositionDirectionShort {
		priceMove = priceMove.Neg()
	}

	return positionDomain.quantity.Mul(priceMove)
}

// ValueAt is what this position is worth to the account at that price: its margin plus
// what it has made or lost. It never falls below nothing — an isolated position loses
// at most its own margin.
func (positionDomain ContractBacktestPositionDomain) ValueAt(price decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Zero, positionDomain.margin.Add(positionDomain.ProfitAt(price)))
}

// ExitOn is the round trip this bar forced, if it forced one.
//
// The side against the position is asked first, and of the two things waiting there —
// the stop, judged on the traded high and low, and liquidation, judged on the mark
// price — **the one nearer the entry is asked first**, because a price moving against
// the position reaches it first. Only then the take profit. A bar reaching both sides
// is read as the side against the position: the high and low cannot say which came
// first, and only this reading never flatters the strategy.
func (positionDomain ContractBacktestPositionDomain) ExitOn(
	bucket dto.KCandleContractDto, exitTime time.Time,
) (vo.ContractClosedTradeVo, bool) {
	isShort := positionDomain.direction == vo.PositionDirectionShort
	liquidationPrice := positionDomain.LiquidationPrice()

	stopReached := positionDomain.exitPrices.HasStopLoss &&
		((!isShort && bucket.Low.LessThanOrEqual(positionDomain.exitPrices.StopLossPrice)) ||
			(isShort && bucket.High.GreaterThanOrEqual(positionDomain.exitPrices.StopLossPrice)))
	liquidationReached := (!isShort && liquidationPrice.IsPositive() &&
		bucket.MarkLow.LessThanOrEqual(liquidationPrice)) ||
		(isShort && bucket.MarkHigh.GreaterThanOrEqual(liquidationPrice))
	stopIsNearer := positionDomain.exitPrices.HasStopLoss &&
		((!isShort && positionDomain.exitPrices.StopLossPrice.GreaterThanOrEqual(liquidationPrice)) ||
			(isShort && positionDomain.exitPrices.StopLossPrice.LessThanOrEqual(liquidationPrice)))

	if stopReached && (stopIsNearer || !liquidationReached) {
		return positionDomain.ClosedAt(exitTime,
			positionDomain.exitFillFor(positionDomain.exitPrices.StopLossPrice),
			vo.TradeExitReasonStopLoss), true
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

// ClosedBySignalAt is the round trip left behind when a signal closes this at that
// bar's close, filled on the wrong side of it by the slippage.
func (positionDomain ContractBacktestPositionDomain) ClosedBySignalAt(
	exitTime time.Time, closePrice decimal.Decimal,
) vo.ContractClosedTradeVo {
	return positionDomain.ClosedAt(
		exitTime, positionDomain.exitFillFor(closePrice), vo.TradeExitReasonSignal)
}

// exitFillFor is what getting out at that price actually fills at: a long sells and a
// short buys back, each on the wrong side of the price by the slippage. It is shared by
// every way out that trades — the stop, the take profit and the signal.
func (positionDomain ContractBacktestPositionDomain) exitFillFor(price decimal.Decimal) decimal.Decimal {
	if positionDomain.direction == vo.PositionDirectionShort {
		return positionDomain.slippage.BuyingAt(price)
	}

	return positionDomain.slippage.SellingAt(price)
}

// ClosedAt turns this position into the round trip it leaves behind.
//
// A liquidated position loses its whole margin and pays nothing more: the venue's
// liquidation fee comes out of that margin, and the funding it paid or received had
// already moved in and out of it. So its loss is exactly what it put down to open —
// the opening margin and the entry charge — and nothing is added to it twice.
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
	closedTrade.Profit = positionDomain.ProfitAt(exitPrice).
		Sub(positionDomain.entryCost).Sub(closedTrade.ExitCost).Sub(positionDomain.fundingFeePaid)

	return closedTrade
}

// CashReturnedFor is what the account gets back for letting this position go: its
// margin and what it made or lost, less the exit charge — and nothing at all when it
// was liquidated. It never goes below nothing.
func (positionDomain ContractBacktestPositionDomain) CashReturnedFor(
	closedTrade vo.ContractClosedTradeVo,
) decimal.Decimal {
	if closedTrade.ExitReason == vo.TradeExitReasonLiquidation {
		return decimal.Zero
	}

	return decimal.Max(decimal.Zero, positionDomain.margin.
		Add(positionDomain.ProfitAt(closedTrade.ExitPrice)).Sub(closedTrade.ExitCost))
}
