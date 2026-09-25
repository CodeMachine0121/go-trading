package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// maximumStakeScale truncates (never rounds) the non-terminating affordable-stake division, so a stake's own cost always fits in the cash.
const maximumStakeScale = 16

// BacktestTransactionCostsDomain holds entry and exit cost rates as percentages of the money changing hands; the zero value pays nothing.
// It only does arithmetic; who pays and when is settled in BacktestAccountDomain.
type BacktestTransactionCostsDomain struct {
	// Zero means no charge; the constructor already defaulted a missing exit rate to the entry rate.
	entryCostPercentage decimal.Decimal
	exitCostPercentage  decimal.Decimal
}

// NewBacktestTransactionCostsDomain defaults a missing exit rate to the entry rate (crypto charges both ways equally; Taiwan adds an exit tax).
// Both rates are validated as declared, before the fallback.
func NewBacktestTransactionCostsDomain(
	entryCostPercentage decimal.Decimal, exitCostPercentage decimal.Decimal,
) (BacktestTransactionCostsDomain, error) {
	if entryError := validatedCostPercentage(
		entryCostPercentage, "進場成本率"); entryError != nil {
		return BacktestTransactionCostsDomain{}, entryError
	}

	if exitError := validatedCostPercentage(
		exitCostPercentage, "出場成本率"); exitError != nil {
		return BacktestTransactionCostsDomain{}, exitError
	}

	if !exitCostPercentage.IsPositive() {
		exitCostPercentage = entryCostPercentage
	}

	return BacktestTransactionCostsDomain{
		entryCostPercentage: entryCostPercentage,
		exitCostPercentage:  exitCostPercentage,
	}, nil
}

// validatedCostPercentage's wording intentionally differs from the exit distances', since an over-100% rate means something different.
func validatedCostPercentage(costPercentage decimal.Decimal, name string) error {
	if costPercentage.IsNegative() {
		return fmt.Errorf("%s不得為負——負的成本等於交易就送錢", name)
	}

	if costPercentage.GreaterThan(oneHundredPercent) {
		return fmt.Errorf("%s不得超過 100%%——成本不會超過成交金額本身", name)
	}

	return nil
}

// MaximumStakeFrom is the largest stake whose entry cost still fits in the cash, shared by every sizing mode.
// With no entry cost it returns the cash untouched rather than dividing by one, which would still truncate.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) MaximumStakeFrom(
	availableCash decimal.Decimal,
) decimal.Decimal {
	if !backtestTransactionCostsDomain.entryCostPercentage.IsPositive() {
		return availableCash
	}

	// cash × 100 ÷ (100 + rate) stays in the caller's units and rounds once, unlike cash ÷ (1 + rate/100).
	return availableCash.Mul(oneHundredPercent).
		DivRound(oneHundredPercent.Add(
			backtestTransactionCostsDomain.entryCostPercentage), maximumStakeScale+2).
		Truncate(maximumStakeScale)
}

func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) EntryCostFor(
	stake decimal.Decimal,
) decimal.Decimal {
	return stake.Mul(backtestTransactionCostsDomain.entryCostPercentage).
		Div(oneHundredPercent)
}

// ExitCostFor charges on units × exit price (not position value, which differs for shorts), and takes the magnitude so a negative price can never turn a cost into income.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) ExitCostFor(
	moneyChangingHands decimal.Decimal,
) decimal.Decimal {
	return moneyChangingHands.Mul(backtestTransactionCostsDomain.exitCostPercentage).
		Div(oneHundredPercent).Abs()
}

// ForMarginAt scales the entry rate by leverage, because the entry charge is on the notional, letting PositionSizingDomain size contract margin with the spot arithmetic.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) ForMarginAt(
	leverage decimal.Decimal,
) BacktestTransactionCostsDomain {
	return BacktestTransactionCostsDomain{
		entryCostPercentage: backtestTransactionCostsDomain.entryCostPercentage.Mul(leverage),
		exitCostPercentage:  backtestTransactionCostsDomain.exitCostPercentage,
	}
}
