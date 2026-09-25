package domains

import (
	"time"

	"github.com/shopspring/decimal"
)

// BacktestPositionTermsDomain bundles sizing, transaction costs and exit levels to answer one question: open a position here with this much cash.
// Its zero value stakes everything, pays nothing and simulates no exits; new terms (e.g. minimum fees) belong here and in OpenFor.
type BacktestPositionTermsDomain struct {
	sizing           PositionSizingDomain
	exitLevels       BacktestExitLevelsDomain
	transactionCosts BacktestTransactionCostsDomain
}

// NewBacktestPositionTermsDomain validates nothing; each part validated itself when built.
func NewBacktestPositionTermsDomain(
	sizing PositionSizingDomain,
	exitLevels BacktestExitLevelsDomain,
	transactionCosts BacktestTransactionCostsDomain,
) BacktestPositionTermsDomain {
	return BacktestPositionTermsDomain{
		sizing:           sizing,
		exitLevels:       exitLevels,
		transactionCosts: transactionCosts,
	}
}

// NeverOpensAnything reports terms that could never open a position, so validation can refuse them upfront.
func (backtestPositionTermsDomain BacktestPositionTermsDomain) NeverOpensAnything() bool {
	return backtestPositionTermsDomain.sizing.NeverStakesUnder(
		backtestPositionTermsDomain.transactionCosts)
}

// OpenFor returns false when the cash cannot cover a stake or the price is zero; the replay just carries on.
func (backtestPositionTermsDomain BacktestPositionTermsDomain) OpenFor(
	entryTime time.Time,
	entryPrice decimal.Decimal,
	availableCash decimal.Decimal,
) (BacktestPositionDomain, bool) {
	stake, canStake := backtestPositionTermsDomain.sizing.StakeFor(
		availableCash, backtestPositionTermsDomain.transactionCosts)
	if !canStake {
		return BacktestPositionDomain{}, false
	}

	return newBacktestPositionDomain(
		entryTime, entryPrice, stake,
		backtestPositionTermsDomain.exitLevels,
		backtestPositionTermsDomain.transactionCosts)
}
