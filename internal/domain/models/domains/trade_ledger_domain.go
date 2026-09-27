package domains

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// averagePriceScale keeps averages exact enough to multiply back into money without drifting a cent.
const averagePriceScale = 12

// TradeLedgerDomain is a trade's fills in the order they happened; it knows no market, so the contract and spot journals share it.
type TradeLedgerDomain struct {
	fills   []vo.TradeLedgerFillVo
	wording vo.TradeLedgerWordingVo
}

// NewTradeLedgerDomain orders entries before exits at the same instant so a same-second round trip never reads as selling what was not held.
func NewTradeLedgerDomain(fills []vo.TradeLedgerFillVo, wording vo.TradeLedgerWordingVo) TradeLedgerDomain {
	orderedFills := slices.Clone(fills)
	slices.SortStableFunc(orderedFills, func(earlier, later vo.TradeLedgerFillVo) int {
		if byTime := earlier.FilledAt.Compare(later.FilledAt); byTime != 0 {
			return byTime
		}

		// "entry" sorts before "exit".
		return cmp.Compare(earlier.Kind, later.Kind)
	})

	return TradeLedgerDomain{fills: orderedFills, wording: wording}
}

func (ledgerDomain TradeLedgerDomain) EnteredQuantity() decimal.Decimal {
	return ledgerDomain.quantityOf(vo.ContractTradeFillKindEntry)
}

func (ledgerDomain TradeLedgerDomain) ExitedQuantity() decimal.Decimal {
	return ledgerDomain.quantityOf(vo.ContractTradeFillKindExit)
}

func (ledgerDomain TradeLedgerDomain) Position() decimal.Decimal {
	return ledgerDomain.EnteredQuantity().Sub(ledgerDomain.ExitedQuantity())
}

func (ledgerDomain TradeLedgerDomain) IsFlat() bool {
	return ledgerDomain.Position().IsZero()
}

func (ledgerDomain TradeLedgerDomain) AverageEntryPrice() decimal.Decimal {
	return ledgerDomain.averagePriceOf(vo.ContractTradeFillKindEntry)
}

// AverageExitPrice is false until something has been sold back.
func (ledgerDomain TradeLedgerDomain) AverageExitPrice() (decimal.Decimal, bool) {
	if ledgerDomain.ExitedQuantity().IsZero() {
		return decimal.Zero, false
	}

	return ledgerDomain.averagePriceOf(vo.ContractTradeFillKindExit), true
}

func (ledgerDomain TradeLedgerDomain) FirstEntryAt() time.Time {
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == vo.ContractTradeFillKindEntry {
			return fill.FilledAt
		}
	}

	return time.Time{}
}

func (ledgerDomain TradeLedgerDomain) FirstEntryPrice() decimal.Decimal {
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == vo.ContractTradeFillKindEntry {
			return fill.Price
		}
	}

	return decimal.Zero
}

func (ledgerDomain TradeLedgerDomain) LastFillAt() time.Time {
	if len(ledgerDomain.fills) == 0 {
		return time.Time{}
	}

	return ledgerDomain.fills[len(ledgerDomain.fills)-1].FilledAt
}

func (ledgerDomain TradeLedgerDomain) TotalFee() decimal.Decimal {
	totalFee := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		totalFee = totalFee.Add(fill.Fee)
	}

	return totalFee
}

// EntryValue is what everything bought or opened cost before fees.
func (ledgerDomain TradeLedgerDomain) EntryValue() decimal.Decimal {
	return ledgerDomain.valueOf(vo.ContractTradeFillKindEntry)
}

// GrossProfit is realised on what has been sold back, measured from the average entry.
func (ledgerDomain TradeLedgerDomain) GrossProfit(direction vo.PositionDirectionVo) decimal.Decimal {
	exitedQuantity := ledgerDomain.ExitedQuantity()
	if exitedQuantity.IsZero() {
		return decimal.Zero
	}

	exitValue := ledgerDomain.valueOf(vo.ContractTradeFillKindExit)
	entryValueOfExited := ledgerDomain.EntryValue().Mul(exitedQuantity).Div(ledgerDomain.EnteredQuantity())
	if direction == vo.PositionDirectionShort {
		return entryValueOfExited.Sub(exitValue)
	}

	return exitValue.Sub(entryValueOfExited)
}

// ProfitAt is what the whole entered quantity would make at a price, which is how excursions are measured.
func (ledgerDomain TradeLedgerDomain) ProfitAt(
	direction vo.PositionDirectionVo, price decimal.Decimal,
) decimal.Decimal {
	valueAtPrice := price.Mul(ledgerDomain.EnteredQuantity())
	if direction == vo.PositionDirectionShort {
		return ledgerDomain.EntryValue().Sub(valueAtPrice)
	}

	return valueAtPrice.Sub(ledgerDomain.EntryValue())
}

// OpenProfitAt is what the quantity still held would make at a price.
func (ledgerDomain TradeLedgerDomain) OpenProfitAt(
	direction vo.PositionDirectionVo, price decimal.Decimal,
) decimal.Decimal {
	difference := price.Sub(ledgerDomain.AverageEntryPrice())
	if direction == vo.PositionDirectionShort {
		difference = difference.Neg()
	}

	return difference.Mul(ledgerDomain.Position())
}

// PositionAt counts fills at the moment itself, as the venue does when funding settles.
func (ledgerDomain TradeLedgerDomain) PositionAt(moment time.Time) decimal.Decimal {
	position := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.FilledAt.After(moment) {
			break
		}
		position = position.Add(ledgerDomain.signedQuantityOf(fill))
	}

	return position
}

// Validate checks the fills as a whole, because a fill that is fine alone can still sell back more than was held.
func (ledgerDomain TradeLedgerDomain) Validate(now time.Time) error {
	wording := ledgerDomain.wording

	for _, fill := range ledgerDomain.fills {
		if fill.Kind != vo.ContractTradeFillKindEntry && fill.Kind != vo.ContractTradeFillKindExit {
			return fmt.Errorf("%w: 只有%s與%s", wording.ValidationError, wording.Entry, wording.Exit)
		}
		if !fill.Price.IsPositive() || !fill.Quantity.IsPositive() {
			return fmt.Errorf("%w: %s與數量必須大於零", wording.ValidationError, wording.Price)
		}
		if fill.Fee.IsNegative() {
			return fmt.Errorf("%w: 手續費不得為負", wording.ValidationError)
		}
		if fill.FilledAt.After(now) {
			return fmt.Errorf("%w: 時間不能在未來", wording.ValidationError)
		}
	}

	if ledgerDomain.EnteredQuantity().IsZero() {
		return fmt.Errorf("%w: 一筆交易至少要有一筆%s；要整筆放棄請刪除交易", wording.ValidationError, wording.Entry)
	}

	firstEntryAt := ledgerDomain.FirstEntryAt()
	position := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == vo.ContractTradeFillKindExit && fill.FilledAt.Before(firstEntryAt) {
			return fmt.Errorf("%w: %s不能早於第一筆%s", wording.ValidationError, wording.Exit, wording.Entry)
		}

		if fill.Kind == vo.ContractTradeFillKindExit && fill.Quantity.GreaterThan(position) {
			return fmt.Errorf("%w: %s數量超過目前%s %s%s",
				wording.ValidationError, wording.Exit, wording.Holding, position.String(), wording.OverExitAdvice)
		}

		position = position.Add(ledgerDomain.signedQuantityOf(fill))
	}

	return nil
}

func (ledgerDomain TradeLedgerDomain) quantityOf(kind vo.ContractTradeFillKindVo) decimal.Decimal {
	quantity := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == kind {
			quantity = quantity.Add(fill.Quantity)
		}
	}

	return quantity
}

func (ledgerDomain TradeLedgerDomain) valueOf(kind vo.ContractTradeFillKindVo) decimal.Decimal {
	value := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == kind {
			value = value.Add(fill.Price.Mul(fill.Quantity))
		}
	}

	return value
}

func (ledgerDomain TradeLedgerDomain) averagePriceOf(kind vo.ContractTradeFillKindVo) decimal.Decimal {
	quantity := ledgerDomain.quantityOf(kind)
	if quantity.IsZero() {
		return decimal.Zero
	}

	return ledgerDomain.valueOf(kind).DivRound(quantity, averagePriceScale)
}

func (ledgerDomain TradeLedgerDomain) signedQuantityOf(fill vo.TradeLedgerFillVo) decimal.Decimal {
	if fill.Kind == vo.ContractTradeFillKindExit {
		return fill.Quantity.Neg()
	}

	return fill.Quantity
}
