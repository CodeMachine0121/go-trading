package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractPositionTermsDomain is the terms one contract replay opens a position on:
// what a spot replay's terms already say — how much of the cash it stakes, what the
// venue charges, where it gets out — plus the leverage, the slippage and the venue's
// trading rules.
//
// Like its spot counterpart it answers a single question: *given this much cash, open a
// position facing this way here.* How much margin that is, whether it is affordable,
// how many units it buys once stepped down, whether the venue would take it, what it
// costs and where it stops are one answer, not steps a caller strings together.
type ContractPositionTermsDomain struct {
	sizing           PositionSizingDomain
	exitLevels       BacktestExitLevelsDomain
	transactionCosts BacktestTransactionCostsDomain
	leverage         decimal.Decimal
	slippage         BacktestSlippageDomain
	tradingRules     ContractTradingRulesDomain
}

// NewContractPositionTermsDomain builds on the spot terms, which have already refused
// what they had to refuse. The leverage arrives read, one or more.
func NewContractPositionTermsDomain(
	positionTerms BacktestPositionTermsDomain,
	leverage decimal.Decimal,
	slippage BacktestSlippageDomain,
	tradingRules ContractTradingRulesDomain,
) ContractPositionTermsDomain {
	return ContractPositionTermsDomain{
		sizing:           positionTerms.sizing,
		exitLevels:       positionTerms.exitLevels,
		transactionCosts: positionTerms.transactionCosts,
		leverage:         leverage,
		slippage:         slippage,
		tradingRules:     tradingRules,
	}
}

// NeverOpensAnything says whether these terms could never put a position down whatever
// the account holds: a percentage that, with its entry charge taken on the notional,
// asks for more than all of the cash.
func (termsDomain ContractPositionTermsDomain) NeverOpensAnything() bool {
	return termsDomain.sizing.NeverStakesUnder(termsDomain.transactionCosts.ForMarginAt(termsDomain.leverage))
}

// OpenFor is the position these terms open facing that way at that bar's close with
// that much cash on hand, and what became of the attempt.
func (termsDomain ContractPositionTermsDomain) OpenFor(
	direction vo.PositionDirectionVo,
	entryTime time.Time,
	closePrice decimal.Decimal,
	availableCash decimal.Decimal,
) (ContractBacktestPositionDomain, vo.ContractOpeningOutcomeVo) {
	entryPrice := termsDomain.slippage.BuyingAt(closePrice)
	if direction == vo.PositionDirectionShort {
		entryPrice = termsDomain.slippage.SellingAt(closePrice)
	}

	margin, canStake := termsDomain.sizing.StakeFor(
		availableCash, termsDomain.transactionCosts.ForMarginAt(termsDomain.leverage))
	if !canStake || !entryPrice.IsPositive() {
		return ContractBacktestPositionDomain{}, vo.ContractOpeningUnaffordable
	}

	quantity := termsDomain.tradingRules.QuantityFor(margin.Mul(termsDomain.leverage), entryPrice)
	if !termsDomain.tradingRules.Admits(quantity, entryPrice, termsDomain.leverage) {
		return ContractBacktestPositionDomain{}, vo.ContractOpeningBlockedByTradingRules
	}

	// The margin actually put down is what the stepped-down quantity needs; whatever
	// the rounding left over stays in the account.
	notional := quantity.Mul(entryPrice)
	actualMargin := notional.Div(termsDomain.leverage)

	exitPrices := termsDomain.exitLevels.PricesFacing(direction, entryPrice)
	exitPrices.StopLossPrice = termsDomain.tradingRules.RoundedToTick(exitPrices.StopLossPrice)
	exitPrices.TakeProfitPrice = termsDomain.tradingRules.RoundedToTick(exitPrices.TakeProfitPrice)

	return ContractBacktestPositionDomain{
		direction:        direction,
		entryTime:        entryTime.UTC(),
		entryPrice:       entryPrice,
		leverage:         termsDomain.leverage,
		quantity:         quantity,
		openingMargin:    actualMargin,
		margin:           actualMargin,
		fundingFeePaid:   decimal.Zero,
		exitPrices:       exitPrices,
		maintenanceTier:  termsDomain.tradingRules.TierFor(notional),
		transactionCosts: termsDomain.transactionCosts,
		slippage:         termsDomain.slippage,
		entryCost:        termsDomain.transactionCosts.EntryCostFor(notional),
	}, vo.ContractOpeningOpened
}
