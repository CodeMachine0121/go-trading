package vo

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractOrderRefusalReasonVo is which of the venue's rules an order breaks.
type ContractOrderRefusalReasonVo string

const (
	// ContractOrderRefusalBelowMinimumQuantity is fewer units, once stepped down, than
	// the smallest order the venue takes.
	ContractOrderRefusalBelowMinimumQuantity ContractOrderRefusalReasonVo = "belowMinimumQuantity"
	// ContractOrderRefusalBelowMinimumNotional is less notional than the smallest order
	// the venue takes.
	ContractOrderRefusalBelowMinimumNotional ContractOrderRefusalReasonVo = "belowMinimumNotional"
	// ContractOrderRefusalAboveTierLeverage is more leverage than the tier the order's
	// notional falls in allows.
	ContractOrderRefusalAboveTierLeverage ContractOrderRefusalReasonVo = "aboveTierLeverage"
)

// ContractOrderRefusalVo is why the venue would not take an order, with the figures
// that say so: what the order came to and the rule it fell short of. Immutable, no
// behavior beyond its shape conversion — which rule an order breaks is ContractTradingRulesDomain.RefusalFor's
// answer.
type ContractOrderRefusalVo struct {
	Reason              ContractOrderRefusalReasonVo
	Quantity            decimal.Decimal
	MinimumQuantity     decimal.Decimal
	Notional            decimal.Decimal
	MinimumNotional     decimal.Decimal
	TierMaximumLeverage int
}

// ToDto is this refusal in the shape the domain hands outwards.
func (contractOrderRefusalVo ContractOrderRefusalVo) ToDto() dto.ContractOrderRefusalDto {
	return dto.ContractOrderRefusalDto{
		Reason:              string(contractOrderRefusalVo.Reason),
		Quantity:            contractOrderRefusalVo.Quantity,
		MinimumQuantity:     contractOrderRefusalVo.MinimumQuantity,
		Notional:            contractOrderRefusalVo.Notional,
		MinimumNotional:     contractOrderRefusalVo.MinimumNotional,
		TierMaximumLeverage: contractOrderRefusalVo.TierMaximumLeverage,
	}
}
