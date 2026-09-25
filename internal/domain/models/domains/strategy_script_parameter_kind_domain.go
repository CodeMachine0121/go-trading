package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableStrategyScriptParameterKinds is every declarable kind, in the order offered back on an unrecognised declaration.
var declarableStrategyScriptParameterKinds = []vo.StrategyScriptParameterKindVo{
	vo.StrategyScriptParameterKindLookbackCount,
	vo.StrategyScriptParameterKindNumber,
	vo.StrategyScriptParameterKindBoolean,
}

// StrategyScriptParameterKindDomain is a declared parameter kind; its zero value is only returned alongside an error.
type StrategyScriptParameterKindDomain struct {
	value vo.StrategyScriptParameterKindVo
}

// NewStrategyScriptParameterKindDomain matches case- and blank-insensitively and refuses an empty declaration rather than defaulting, since a wrong kind reads the wrong number of candles.
func NewStrategyScriptParameterKindDomain(declared string) (StrategyScriptParameterKindDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)

	for _, declarableKind := range declarableStrategyScriptParameterKinds {
		if strings.EqualFold(string(declarableKind), normalizedDeclaration) {
			return StrategyScriptParameterKindDomain{value: declarableKind}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableStrategyScriptParameterKinds))
	for _, declarableKind := range declarableStrategyScriptParameterKinds {
		declarableSpellings = append(declarableSpellings, string(declarableKind))
	}

	return StrategyScriptParameterKindDomain{}, fmt.Errorf(
		"參數種類 %q 不在可宣告的種類之內：%s",
		declared, strings.Join(declarableSpellings, "、"))
}

func (strategyScriptParameterKindDomain StrategyScriptParameterKindDomain) Value() vo.StrategyScriptParameterKindVo {
	return strategyScriptParameterKindDomain.value
}
