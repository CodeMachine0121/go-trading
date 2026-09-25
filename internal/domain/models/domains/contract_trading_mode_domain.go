package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableContractTradingModes is in the order offered back when a declaration is not recognised.
var declarableContractTradingModes = []vo.ContractTradingModeVo{
	vo.ContractTradingModeLongShort,
	vo.ContractTradingModeLongOnly,
	vo.ContractTradingModeShortOnly,
}

// ContractTradingModeDomain decides what a signal asks a contract account to hold; spot is refused rather than read as long only.
type ContractTradingModeDomain struct {
	value vo.ContractTradingModeVo
}

// NewContractTradingModeDomain defaults a blank declaration to long-short and matches case-insensitively.
func NewContractTradingModeDomain(declared string) (ContractTradingModeDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return ContractTradingModeDomain{value: vo.ContractTradingModeLongShort}, nil
	}

	for _, declarableMode := range declarableContractTradingModes {
		if strings.EqualFold(string(declarableMode), normalizedDeclaration) {
			return ContractTradingModeDomain{value: declarableMode}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableContractTradingModes))
	for _, declarableMode := range declarableContractTradingModes {
		declarableSpellings = append(declarableSpellings, string(declarableMode))
	}

	return ContractTradingModeDomain{}, fmt.Errorf(
		"合約的交易模式只能是 %s 其中之一（多空反手、只做多、只做空）",
		strings.Join(declarableSpellings, "、"))
}

func (contractTradingModeDomain ContractTradingModeDomain) Value() vo.ContractTradingModeVo {
	return contractTradingModeDomain.value
}

// TargetFor maps a signal to a target position, going flat when the mode cannot face the signal's direction.
func (contractTradingModeDomain ContractTradingModeDomain) TargetFor(
	signal SignalDomain,
) vo.TargetPositionVo {
	switch signal.Value() {
	case vo.SignalBuy:
		if contractTradingModeDomain.value == vo.ContractTradingModeShortOnly {
			return vo.TargetPositionFlat
		}

		return vo.TargetPositionLong
	case vo.SignalSell:
		if contractTradingModeDomain.value == vo.ContractTradingModeLongOnly {
			return vo.TargetPositionFlat
		}

		return vo.TargetPositionShort
	}

	return vo.TargetPositionUnchanged
}
