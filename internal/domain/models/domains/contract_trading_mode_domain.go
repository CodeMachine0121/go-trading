package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableContractTradingModes is the entire set a contract replay may trade by, in
// the order it is offered back when a declaration is not recognised.
var declarableContractTradingModes = []vo.ContractTradingModeVo{
	vo.ContractTradingModeLongShort,
	vo.ContractTradingModeLongOnly,
	vo.ContractTradingModeShortOnly,
}

// ContractTradingModeDomain is which rules a contract account trades by, and the one
// thing that follows from it: what a signal asks the account to be holding.
//
// There is no spot among them. Spot is cash for goods, which is the spot replay's
// business; asking for it here is asking the contract account to be something it is
// not, and is refused rather than quietly read as long only.
type ContractTradingModeDomain struct {
	value vo.ContractTradingModeVo
}

// NewContractTradingModeDomain reads what was declared. Declaring nothing is long and
// short — the natural way to trade a perpetual contract, and what a trading mode has
// always meant when left blank. Spelling is forgiving about blanks and letter case.
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

// TargetFor is what this signal asks a contract account to be holding under these
// rules. A hold asks for nothing; a buy or a sell asks either to face that way or,
// where these rules cannot face that way, to get out.
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
