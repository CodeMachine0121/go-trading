package vo

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

type ContractOrderRefusalReasonVo string

const (
	// ContractOrderRefusalBelowMinimumQuantity applies after the quantity is stepped down.
	ContractOrderRefusalBelowMinimumQuantity ContractOrderRefusalReasonVo = "belowMinimumQuantity"
	ContractOrderRefusalBelowMinimumNotional ContractOrderRefusalReasonVo = "belowMinimumNotional"
	ContractOrderRefusalAboveTierLeverage    ContractOrderRefusalReasonVo = "aboveTierLeverage"
)

// ContractOrderRefusalVo is why the venue would refuse an order, with the figures involved; see ContractTradingRulesDomain.RefusalFor.
type ContractOrderRefusalVo struct {
	Reason              ContractOrderRefusalReasonVo
	Quantity            decimal.Decimal
	MinimumQuantity     decimal.Decimal
	Notional            decimal.Decimal
	MinimumNotional     decimal.Decimal
	TierMaximumLeverage int
}

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
