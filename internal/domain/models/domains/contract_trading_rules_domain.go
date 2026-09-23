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
	// unsteppedQuantityScale is how finely a quantity is kept for a symbol whose
	// specification names no quantity step — fine enough never to be the reason two
	// figures disagree.
	unsteppedQuantityScale = 16
)

// ContractTradingRulesDomain is what the venue lets a contract replay do on one symbol:
// the trading specification (how prices and quantities step, how small an order may
// be) and the maintenance margin ladder (how much margin a position of a given size
// must keep, and how much leverage it may carry).
//
// A replay cannot pretend these away. An order the venue would have refused is not
// opened here either, and a position is liquidated where the venue would have
// liquidated it — which is the whole reason a contract replay exists.
type ContractTradingRulesDomain struct {
	tickSize        decimal.Decimal
	quantityStep    decimal.Decimal
	minimumQuantity decimal.Decimal
	minimumNotional decimal.Decimal
	// tiers is the full ladder, smallest first. When the symbol has none it holds a
	// single tier standing in for every size, taken from the specification.
	tiers             []vo.ContractMaintenanceMarginTierVo
	hasLadder         bool
	ladderConfirmedAt time.Time
}

// NewContractTradingRulesDomain reads the symbol's specification and ladder as they
// are stored. A symbol the system does not know, or knows without a specification, is
// refused: without its maintenance margin rate nobody can say when a position on it
// stops holding, and that is the question a contract replay is asked.
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

// MaximumLeverage is the most leverage any position on this symbol may carry — what
// its smallest tier allows — and whether there is such a limit at all. Without a
// ladder nothing here says, and the replay does not invent one.
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

// QuantityFor is how many units that much notional buys at that price, stepped down to
// what the venue accepts. Rounding down is the only direction that never spends money
// the margin was not asked for. The price is positive: an opening at a price of
// nothing is refused before anyone asks how many units it buys.
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

// Admits says whether the venue would take an order of that many units at that price
// carrying that much leverage: at least the minimum quantity, at least the minimum
// notional, and no more leverage than the tier its notional falls in allows.
func (tradingRulesDomain ContractTradingRulesDomain) Admits(
	quantity decimal.Decimal, price decimal.Decimal, leverage decimal.Decimal,
) bool {
	if !quantity.IsPositive() || quantity.LessThan(tradingRulesDomain.minimumQuantity) {
		return false
	}

	notional := quantity.Mul(price)
	if notional.LessThan(tradingRulesDomain.minimumNotional) {
		return false
	}

	if !tradingRulesDomain.hasLadder {
		return true
	}

	return leverage.LessThanOrEqual(decimal.NewFromInt(int64(
		tradingRulesDomain.TierFor(notional).MaximumLeverage)))
}

// TierFor is the tier a position of that notional falls in: the first whose cap it
// does not exceed, or the last one when it is bigger than every cap. Without a ladder
// it is always the specification's smallest tier.
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

// RoundedToTick is that price moved to the nearest price the venue can quote. A symbol
// whose specification names no tick leaves it where it is.
func (tradingRulesDomain ContractTradingRulesDomain) RoundedToTick(price decimal.Decimal) decimal.Decimal {
	if !tradingRulesDomain.tickSize.IsPositive() {
		return price
	}

	return price.Div(tradingRulesDomain.tickSize).Round(0).Mul(tradingRulesDomain.tickSize)
}

// MaintenanceMarginBasisDto is which figures these rules hold, in the shape the report
// card says it in.
func (tradingRulesDomain ContractTradingRulesDomain) MaintenanceMarginBasisDto() dto.MaintenanceMarginBasisDto {
	if !tradingRulesDomain.hasLadder {
		return dto.MaintenanceMarginBasisDto{Kind: maintenanceMarginBasisSmallestTier}
	}

	confirmedAt := tradingRulesDomain.ladderConfirmedAt

	return dto.MaintenanceMarginBasisDto{Kind: maintenanceMarginBasisTiers, ConfirmedAt: &confirmedAt}
}
