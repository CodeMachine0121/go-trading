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

// ContractMaintenanceMarginLadderDomain is one contract's whole maintenance margin
// ladder, checked as a whole. An instance only exists when every tier makes sense on
// its own and the tiers make sense together: at least one, in order, and none covering
// a stretch of notional another one already covers.
//
// **It is judged whole because it is used whole.** A position is placed on the ladder
// by its notional; a ladder with a gap or an overlap puts some positions on no tier or
// on two, and a replay reading it would pick one without saying so.
type ContractMaintenanceMarginLadderDomain struct {
	symbol string
	tiers  []vo.ContractMaintenanceMarginTierVo
}

// NewContractMaintenanceMarginLadderDomain checks one reported ladder.
func NewContractMaintenanceMarginLadderDomain(
	ladder vo.ContractMaintenanceMarginLadderVo,
) (ContractMaintenanceMarginLadderDomain, error) {
	contractSymbol, symbolError := NewTradingSymbolDomain(ladder.Symbol)
	if symbolError != nil {
		return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
			"%w: %w", ErrContractMaintenanceMarginTierValidation, symbolError)
	}

	if len(ladder.Tiers) == 0 {
		return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
			"%w: 至少要有一級", ErrContractMaintenanceMarginTierValidation)
	}

	tiers := slices.Clone(ladder.Tiers)
	slices.SortFunc(tiers, func(earlier, later vo.ContractMaintenanceMarginTierVo) int {
		return cmp.Compare(earlier.Tier, later.Tier)
	})

	for index, tier := range tiers {
		if tier.Tier < 1 {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 級數至少是第一級", ErrContractMaintenanceMarginTierValidation)
		}
		if tier.NotionalFloor.IsNegative() {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級的名目下限不得為負", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}
		if !tier.NotionalCap.GreaterThan(tier.NotionalFloor) {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級的名目上限必須大於下限", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}
		if tier.MaintenanceMarginRate.IsNegative() || tier.MaintenanceMarginRate.GreaterThan(decimal.NewFromInt(1)) {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級的維持保證金率必須介於零與一之間", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}
		if tier.MaintenanceAmount.IsNegative() {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級的維持保證金速算額不得為負", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}
		if tier.MaximumLeverage < 1 {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級的最高槓桿至少一倍", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}

		if index == 0 {
			continue
		}
		previous := tiers[index-1]
		if tier.Tier == previous.Tier {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級出現兩次", ErrContractMaintenanceMarginTierValidation, tier.Tier)
		}
		if tier.NotionalFloor.LessThan(previous.NotionalCap) {
			return ContractMaintenanceMarginLadderDomain{}, fmt.Errorf(
				"%w: 第 %d 級與第 %d 級的分級重疊", ErrContractMaintenanceMarginTierValidation,
				previous.Tier, tier.Tier)
		}
	}

	return ContractMaintenanceMarginLadderDomain{symbol: contractSymbol.Value(), tiers: tiers}, nil
}

// Symbol is the contract this ladder belongs to.
func (ladderDomain ContractMaintenanceMarginLadderDomain) Symbol() string {
	return ladderDomain.symbol
}

// ToEntities converts this ladder into the tiers that are stored, every one stamped
// with when the ladder was confirmed.
func (ladderDomain ContractMaintenanceMarginLadderDomain) ToEntities(
	confirmedAt time.Time,
) []entities.ContractMaintenanceMarginTier {
	storedTiers := make([]entities.ContractMaintenanceMarginTier, 0, len(ladderDomain.tiers))
	for _, tier := range ladderDomain.tiers {
		storedTiers = append(storedTiers, entities.ContractMaintenanceMarginTier{
			Symbol:                ladderDomain.symbol,
			Tier:                  tier.Tier,
			NotionalFloor:         tier.NotionalFloor,
			NotionalCap:           tier.NotionalCap,
			MaintenanceMarginRate: tier.MaintenanceMarginRate,
			MaintenanceAmount:     tier.MaintenanceAmount,
			MaximumLeverage:       tier.MaximumLeverage,
			ConfirmedAt:           confirmedAt.UTC(),
		})
	}

	return storedTiers
}
