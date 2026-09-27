package domains

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// averagePriceScale keeps averages exact enough to multiply back into money without drifting a cent.
const averagePriceScale = 12

// ContractTradeLedgerDomain is a trade's fills in the order they happened; it knows nothing about contracts, so a spot journal can reuse it.
type ContractTradeLedgerDomain struct {
	fills []entities.ContractTradeFill
}

// NewContractTradeLedgerDomain orders entries before exits at the same instant so a same-second round trip never reads as a short position.
func NewContractTradeLedgerDomain(fills []entities.ContractTradeFill) ContractTradeLedgerDomain {
	orderedFills := slices.Clone(fills)
	slices.SortStableFunc(orderedFills, func(earlier, later entities.ContractTradeFill) int {
		if byTime := earlier.FilledAt.Compare(later.FilledAt); byTime != 0 {
			return byTime
		}

		// "entry" sorts before "exit".
		return cmp.Compare(earlier.Kind, later.Kind)
	})

	return ContractTradeLedgerDomain{fills: orderedFills}
}

func (ledgerDomain ContractTradeLedgerDomain) Fills() []entities.ContractTradeFill {
	return slices.Clone(ledgerDomain.fills)
}

func (ledgerDomain ContractTradeLedgerDomain) EnteredQuantity() decimal.Decimal {
	return ledgerDomain.quantityOf(vo.ContractTradeFillKindEntry)
}

func (ledgerDomain ContractTradeLedgerDomain) ExitedQuantity() decimal.Decimal {
	return ledgerDomain.quantityOf(vo.ContractTradeFillKindExit)
}

func (ledgerDomain ContractTradeLedgerDomain) Position() decimal.Decimal {
	return ledgerDomain.EnteredQuantity().Sub(ledgerDomain.ExitedQuantity())
}

func (ledgerDomain ContractTradeLedgerDomain) IsFlat() bool {
	return ledgerDomain.Position().IsZero()
}

func (ledgerDomain ContractTradeLedgerDomain) AverageEntryPrice() decimal.Decimal {
	return ledgerDomain.averagePriceOf(vo.ContractTradeFillKindEntry)
}

// AverageExitPrice is false until something has been sold back.
func (ledgerDomain ContractTradeLedgerDomain) AverageExitPrice() (decimal.Decimal, bool) {
	if ledgerDomain.ExitedQuantity().IsZero() {
		return decimal.Zero, false
	}

	return ledgerDomain.averagePriceOf(vo.ContractTradeFillKindExit), true
}

func (ledgerDomain ContractTradeLedgerDomain) FirstEntryAt() time.Time {
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(vo.ContractTradeFillKindEntry) {
			return fill.FilledAt
		}
	}

	return time.Time{}
}

func (ledgerDomain ContractTradeLedgerDomain) FirstEntryPrice() decimal.Decimal {
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(vo.ContractTradeFillKindEntry) {
			return fill.Price
		}
	}

	return decimal.Zero
}

func (ledgerDomain ContractTradeLedgerDomain) LastFillAt() time.Time {
	if len(ledgerDomain.fills) == 0 {
		return time.Time{}
	}

	return ledgerDomain.fills[len(ledgerDomain.fills)-1].FilledAt
}

func (ledgerDomain ContractTradeLedgerDomain) TotalFee() decimal.Decimal {
	totalFee := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		totalFee = totalFee.Add(fill.Fee)
	}

	return totalFee
}

func (ledgerDomain ContractTradeLedgerDomain) FeeRateMissing() bool {
	return slices.ContainsFunc(ledgerDomain.fills, func(fill entities.ContractTradeFill) bool {
		return fill.FeeRateMissing
	})
}

// GrossProfit is realised on what has been sold back, measured from the average entry.
func (ledgerDomain ContractTradeLedgerDomain) GrossProfit(direction vo.PositionDirectionVo) decimal.Decimal {
	exitedQuantity := ledgerDomain.ExitedQuantity()
	if exitedQuantity.IsZero() {
		return decimal.Zero
	}

	exitValue := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(vo.ContractTradeFillKindExit) {
			exitValue = exitValue.Add(fill.Price.Mul(fill.Quantity))
		}
	}

	entryValueOfExited := ledgerDomain.entryValue().Mul(exitedQuantity).Div(ledgerDomain.EnteredQuantity())
	if direction == vo.PositionDirectionShort {
		return entryValueOfExited.Sub(exitValue)
	}

	return exitValue.Sub(entryValueOfExited)
}

// ProfitAt is what the whole entered quantity would make at a price, which is how excursions are measured.
func (ledgerDomain ContractTradeLedgerDomain) ProfitAt(
	direction vo.PositionDirectionVo, price decimal.Decimal,
) decimal.Decimal {
	valueAtPrice := price.Mul(ledgerDomain.EnteredQuantity())
	if direction == vo.PositionDirectionShort {
		return ledgerDomain.entryValue().Sub(valueAtPrice)
	}

	return valueAtPrice.Sub(ledgerDomain.entryValue())
}

// OpenProfitAt is what the quantity still held would make at a price.
func (ledgerDomain ContractTradeLedgerDomain) OpenProfitAt(
	direction vo.PositionDirectionVo, price decimal.Decimal,
) decimal.Decimal {
	difference := price.Sub(ledgerDomain.AverageEntryPrice())
	if direction == vo.PositionDirectionShort {
		difference = difference.Neg()
	}

	return difference.Mul(ledgerDomain.Position())
}

// PositionAt counts fills at the moment itself, as the venue does when funding settles.
func (ledgerDomain ContractTradeLedgerDomain) PositionAt(moment time.Time) decimal.Decimal {
	position := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.FilledAt.After(moment) {
			break
		}
		position = position.Add(ledgerDomain.signedQuantityOf(fill))
	}

	return position
}

// Admit adds a fill, refusing it when the trade would stop making sense.
func (ledgerDomain ContractTradeLedgerDomain) Admit(
	fill entities.ContractTradeFill, now time.Time,
) (ContractTradeLedgerDomain, error) {
	return ledgerDomain.settled(append(slices.Clone(ledgerDomain.fills), fill), now)
}

func (ledgerDomain ContractTradeLedgerDomain) Amend(
	fillID uint, fill entities.ContractTradeFill, now time.Time,
) (ContractTradeLedgerDomain, error) {
	index := ledgerDomain.indexOf(fillID)
	if index < 0 {
		return ContractTradeLedgerDomain{}, fmt.Errorf(
			"%w: 這筆交易沒有識別碼為 %d 的成交", ErrContractTradeValidation, fillID)
	}

	amendedFills := slices.Clone(ledgerDomain.fills)
	fill.ID = fillID
	amendedFills[index] = fill

	return ledgerDomain.settled(amendedFills, now)
}

func (ledgerDomain ContractTradeLedgerDomain) Remove(
	fillID uint, now time.Time,
) (ContractTradeLedgerDomain, error) {
	index := ledgerDomain.indexOf(fillID)
	if index < 0 {
		return ContractTradeLedgerDomain{}, fmt.Errorf(
			"%w: 這筆交易沒有識別碼為 %d 的成交", ErrContractTradeValidation, fillID)
	}

	return ledgerDomain.settled(slices.Delete(slices.Clone(ledgerDomain.fills), index, index+1), now)
}

// settled checks the fills as a whole, because a fill that is fine alone can still sell back more than was held.
func (ledgerDomain ContractTradeLedgerDomain) settled(
	fills []entities.ContractTradeFill, now time.Time,
) (ContractTradeLedgerDomain, error) {
	for _, fill := range fills {
		if fill.Kind != string(vo.ContractTradeFillKindEntry) && fill.Kind != string(vo.ContractTradeFillKindExit) {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 成交只有進場（entry）與出場（exit）", ErrContractTradeValidation)
		}
		if fill.Liquidity != string(vo.TradeFillLiquidityMaker) && fill.Liquidity != string(vo.TradeFillLiquidityTaker) {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 成交方式只有掛單（maker）與吃單（taker）", ErrContractTradeValidation)
		}
		if !fill.Price.IsPositive() || !fill.Quantity.IsPositive() {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 成交價與數量必須大於零", ErrContractTradeValidation)
		}
		if fill.Fee.IsNegative() {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 手續費不得為負", ErrContractTradeValidation)
		}
		if fill.FilledAt.After(now) {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 成交時間不能在未來", ErrContractTradeValidation)
		}
	}

	settledLedger := NewContractTradeLedgerDomain(fills)
	if settledLedger.EnteredQuantity().IsZero() {
		return ContractTradeLedgerDomain{}, fmt.Errorf(
			"%w: 一筆交易至少要有一筆進場成交；要整筆放棄請刪除交易", ErrContractTradeValidation)
	}

	firstEntryAt := settledLedger.FirstEntryAt()
	position := decimal.Zero
	for _, fill := range settledLedger.fills {
		if fill.Kind == string(vo.ContractTradeFillKindExit) && fill.FilledAt.Before(firstEntryAt) {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 出場不能早於第一筆進場", ErrContractTradeValidation)
		}

		if fill.Kind == string(vo.ContractTradeFillKindExit) && fill.Quantity.GreaterThan(position) {
			return ContractTradeLedgerDomain{}, fmt.Errorf(
				"%w: 出場數量超過目前持倉 %s，要反手請先平倉再新增一筆反方向的交易",
				ErrContractTradeValidation, position.String())
		}

		position = position.Add(ledgerDomain.signedQuantityOf(fill))
	}

	return settledLedger, nil
}

func (ledgerDomain ContractTradeLedgerDomain) indexOf(fillID uint) int {
	return slices.IndexFunc(ledgerDomain.fills, func(fill entities.ContractTradeFill) bool {
		return fill.ID == fillID
	})
}

func (ledgerDomain ContractTradeLedgerDomain) quantityOf(kind vo.ContractTradeFillKindVo) decimal.Decimal {
	quantity := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(kind) {
			quantity = quantity.Add(fill.Quantity)
		}
	}

	return quantity
}

func (ledgerDomain ContractTradeLedgerDomain) averagePriceOf(kind vo.ContractTradeFillKindVo) decimal.Decimal {
	quantity := ledgerDomain.quantityOf(kind)
	if quantity.IsZero() {
		return decimal.Zero
	}

	value := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(kind) {
			value = value.Add(fill.Price.Mul(fill.Quantity))
		}
	}

	return value.DivRound(quantity, averagePriceScale)
}

func (ledgerDomain ContractTradeLedgerDomain) entryValue() decimal.Decimal {
	value := decimal.Zero
	for _, fill := range ledgerDomain.fills {
		if fill.Kind == string(vo.ContractTradeFillKindEntry) {
			value = value.Add(fill.Price.Mul(fill.Quantity))
		}
	}

	return value
}

func (ledgerDomain ContractTradeLedgerDomain) signedQuantityOf(fill entities.ContractTradeFill) decimal.Decimal {
	if fill.Kind == string(vo.ContractTradeFillKindExit) {
		return fill.Quantity.Neg()
	}

	return fill.Quantity
}
