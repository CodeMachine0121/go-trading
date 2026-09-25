package domains

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	maintenanceMarginBasisTiers        = "tiers"
	maintenanceMarginBasisSmallestTier = "smallestTier"
	// unsteppedQuantityScale is the precision kept when a specification names no quantity step.
	unsteppedQuantityScale = 16
)

// ContractTradingRulesDomain is the venue's trading specification and maintenance margin ladder for one symbol, so a replay refuses and liquidates exactly where the venue would.
type ContractTradingRulesDomain struct {
	tickSize        decimal.Decimal
	quantityStep    decimal.Decimal
	minimumQuantity decimal.Decimal
	minimumNotional decimal.Decimal
	// tiers is sorted smallest first; without a ladder it holds one stand-in tier from the specification.
	tiers             []vo.ContractMaintenanceMarginTierVo
	hasLadder         bool
	ladderConfirmedAt time.Time
}

// NewContractTradingRulesDomain refuses an unknown symbol or one without a specification, since without its maintenance margin rate liquidation cannot be judged.
func NewContractTradingRulesDomain(
	contractTradingSymbol entities.ContractTradingSymbol,
	isRegistered bool,
	maintenanceMarginTiers []entities.ContractMaintenanceMarginTier,
) (ContractTradingRulesDomain, error) {
	if !isRegistered || contractTradingSymbol.SpecificationUpdatedAt == nil ||
		!contractTradingSymbol.MaintenanceMarginRate.Valid {
		return ContractTradingRulesDomain{}, fmt.Errorf(
			"%w: 這個合約標的還沒有交易規格，說不出它的倉位什麼時候撐不住——"+
				"請先把它加入合約追蹤名單，等規格刷新之後再重演", ErrBacktestValidation)
	}

	tradingRulesDomain := ContractTradingRulesDomain{
		tickSize:        contractTradingSymbol.TickSize.Decimal,
		quantityStep:    contractTradingSymbol.QuantityStep.Decimal,
		minimumQuantity: contractTradingSymbol.MinimumQuantity.Decimal,
		minimumNotional: contractTradingSymbol.MinimumNotional.Decimal,
	}

	if len(maintenanceMarginTiers) == 0 {
		tradingRulesDomain.tiers = []vo.ContractMaintenanceMarginTierVo{{
			Tier:                  1,
			MaintenanceMarginRate: contractTradingSymbol.MaintenanceMarginRate.Decimal,
		}}

		return tradingRulesDomain, nil
	}

	tiers := make([]vo.ContractMaintenanceMarginTierVo, 0, len(maintenanceMarginTiers))
	for _, maintenanceMarginTier := range maintenanceMarginTiers {
		tiers = append(tiers, vo.ContractMaintenanceMarginTierVo{
			Tier:                  maintenanceMarginTier.Tier,
			NotionalFloor:         maintenanceMarginTier.NotionalFloor,
			NotionalCap:           maintenanceMarginTier.NotionalCap,
			MaintenanceMarginRate: maintenanceMarginTier.MaintenanceMarginRate,
			MaintenanceAmount:     maintenanceMarginTier.MaintenanceAmount,
			MaximumLeverage:       maintenanceMarginTier.MaximumLeverage,
		})
		tradingRulesDomain.ladderConfirmedAt = maintenanceMarginTier.ConfirmedAt.UTC()
	}
	slices.SortFunc(tiers, func(earlier, later vo.ContractMaintenanceMarginTierVo) int {
		return cmp.Compare(earlier.Tier, later.Tier)
	})

	tradingRulesDomain.tiers = tiers
	tradingRulesDomain.hasLadder = true

	return tradingRulesDomain, nil
}

// MaximumLeverage is the smallest tier's limit; false means there is no ladder and so no known limit.
func (tradingRulesDomain ContractTradingRulesDomain) MaximumLeverage() (int, bool) {
	if !tradingRulesDomain.hasLadder {
		return 0, false
	}

	maximumLeverage := 0
	for _, tier := range tradingRulesDomain.tiers {
		maximumLeverage = max(maximumLeverage, tier.MaximumLeverage)
	}

	return maximumLeverage, true
}

// QuantityFor steps the quantity down to what the venue accepts, since rounding down never spends margin that was not asked for; price must be positive.
func (tradingRulesDomain ContractTradingRulesDomain) QuantityFor(
	notional decimal.Decimal, price decimal.Decimal,
) decimal.Decimal {
	if !tradingRulesDomain.quantityStep.IsPositive() {
		return notional.DivRound(price, unsteppedQuantityScale+2).Truncate(unsteppedQuantityScale)
	}

	stepCount := notional.DivRound(
		price.Mul(tradingRulesDomain.quantityStep), unsteppedQuantityScale).Floor()

	return stepCount.Mul(tradingRulesDomain.quantityStep)
}

func (tradingRulesDomain ContractTradingRulesDomain) Admits(
	quantity decimal.Decimal, price decimal.Decimal, leverage decimal.Decimal,
) bool {
	_, isRefused := tradingRulesDomain.RefusalFor(quantity, price, leverage)

	return !isRefused
}

// RefusalFor checks minimum quantity, then minimum notional, then the tier's leverage limit, naming the first rule broken.
func (tradingRulesDomain ContractTradingRulesDomain) RefusalFor(
	quantity decimal.Decimal, price decimal.Decimal, leverage decimal.Decimal,
) (vo.ContractOrderRefusalVo, bool) {
	notional := quantity.Mul(price)
	refusal := vo.ContractOrderRefusalVo{
		Quantity:        quantity,
		MinimumQuantity: tradingRulesDomain.minimumQuantity,
		Notional:        notional,
		MinimumNotional: tradingRulesDomain.minimumNotional,
	}

	if !quantity.IsPositive() || quantity.LessThan(tradingRulesDomain.minimumQuantity) {
		refusal.Reason = vo.ContractOrderRefusalBelowMinimumQuantity

		return refusal, true
	}

	if notional.LessThan(tradingRulesDomain.minimumNotional) {
		refusal.Reason = vo.ContractOrderRefusalBelowMinimumNotional

		return refusal, true
	}

	if !tradingRulesDomain.hasLadder {
		return vo.ContractOrderRefusalVo{}, false
	}

	tierMaximumLeverage := tradingRulesDomain.TierFor(notional).MaximumLeverage
	if leverage.GreaterThan(decimal.NewFromInt(int64(tierMaximumLeverage))) {
		refusal.Reason = vo.ContractOrderRefusalAboveTierLeverage
		refusal.TierMaximumLeverage = tierMaximumLeverage

		return refusal, true
	}

	return vo.ContractOrderRefusalVo{}, false
}

// HasLadder reports whether the full ladder is held rather than the specification's single stand-in tier.
func (tradingRulesDomain ContractTradingRulesDomain) HasLadder() bool {
	return tradingRulesDomain.hasLadder
}

// TierFor is the first tier whose cap the notional does not exceed, or the last tier when it exceeds every cap.
func (tradingRulesDomain ContractTradingRulesDomain) TierFor(
	notional decimal.Decimal,
) vo.ContractMaintenanceMarginTierVo {
	for _, tier := range tradingRulesDomain.tiers {
		if notional.LessThanOrEqual(tier.NotionalCap) {
			return tier
		}
	}

	return tradingRulesDomain.tiers[len(tradingRulesDomain.tiers)-1]
}

// RoundedToTick rounds to the nearest quotable price, leaving it unchanged when no tick is specified.
func (tradingRulesDomain ContractTradingRulesDomain) RoundedToTick(price decimal.Decimal) decimal.Decimal {
	if !tradingRulesDomain.tickSize.IsPositive() {
		return price
	}

	return price.Div(tradingRulesDomain.tickSize).Round(0).Mul(tradingRulesDomain.tickSize)
}

func (tradingRulesDomain ContractTradingRulesDomain) MaintenanceMarginBasisDto() dto.MaintenanceMarginBasisDto {
	if !tradingRulesDomain.hasLadder {
		return dto.MaintenanceMarginBasisDto{Kind: maintenanceMarginBasisSmallestTier}
	}

	confirmedAt := tradingRulesDomain.ladderConfirmedAt

	return dto.MaintenanceMarginBasisDto{Kind: maintenanceMarginBasisTiers, ConfirmedAt: &confirmedAt}
}
