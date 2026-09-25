package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractPositionTermsDomain is the spot position terms plus leverage, slippage and the venue's trading rules, answering in one call how a contract position opens.
type ContractPositionTermsDomain struct {
	sizing           PositionSizingDomain
	exitLevels       BacktestExitLevelsDomain
	transactionCosts BacktestTransactionCostsDomain
	leverage         decimal.Decimal
	slippage         BacktestSlippageDomain
	tradingRules     ContractTradingRulesDomain
}

// NewContractPositionTermsDomain builds on already-validated spot terms; leverage is at least one.
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

// NeverOpensAnything reports whether the percentage plus its entry charge on the notional would ask for more than all of the cash.
func (termsDomain ContractPositionTermsDomain) NeverOpensAnything() bool {
	return termsDomain.sizing.NeverStakesUnder(termsDomain.transactionCosts.ForMarginAt(termsDomain.leverage))
}

// OpenFor opens a position facing direction at the bar's close with the available cash and reports the outcome.
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

	// Margin is recomputed from the stepped-down quantity; the rounding remainder stays in the account.
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
