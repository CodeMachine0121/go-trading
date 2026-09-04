package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableStrategyParameterKinds is the entire set a caller may declare, in the
// order it is offered back when a declaration is not recognised.
//
// Adding a kind means adding it here, and — if the system is to read any meaning
// into it — saying so where that meaning is read.
//
// A yes-or-no was once noted here as needing more than this list, on the reasoning
// that the value is one number. It does not: zero is no and anything else is yes, so
// the value stays one number and the kind still only says how to read it. What the
// note was right to refuse is widening the stored value to hold anything, because
// that is what would put the kind back at reading time.
var declarableStrategyParameterKinds = []vo.StrategyParameterKindVo{
	vo.StrategyParameterKindLookbackCount,
	vo.StrategyParameterKindNumber,
	vo.StrategyParameterKindBoolean,
}

// StrategyParameterKindDomain is a declared parameter kind. It answers the one
// question the rest of the system asks — is this a look-back count — because that is
// the only kind the system reads any meaning into.
//
// Its zero value is not a usable kind; it is only ever returned alongside an error.
type StrategyParameterKindDomain struct {
	value vo.StrategyParameterKindVo
}

// NewStrategyParameterKindDomain reads what was declared. Spelling is forgiving about
// surrounding blanks and letter case; anything else is refused, naming what could
// have been declared instead.
//
// Declaring nothing is refused rather than defaulted. A kind left out is not an
// omission the system can fill in: the two kinds behave differently in the one way
// that matters, and guessing wrong means reading the wrong number of candles.
func NewStrategyParameterKindDomain(declared string) (StrategyParameterKindDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)

	for _, declarableKind := range declarableStrategyParameterKinds {
		if strings.EqualFold(string(declarableKind), normalizedDeclaration) {
			return StrategyParameterKindDomain{value: declarableKind}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableStrategyParameterKinds))
	for _, declarableKind := range declarableStrategyParameterKinds {
		declarableSpellings = append(declarableSpellings, string(declarableKind))
	}

	return StrategyParameterKindDomain{}, fmt.Errorf(
		"參數種類 %q 不在可宣告的種類之內：%s",
		declared, strings.Join(declarableSpellings, "、"))
}

// Value hands the kind back as it was settled, for storing.
func (strategyParameterKindDomain StrategyParameterKindDomain) Value() vo.StrategyParameterKindVo {
	return strategyParameterKindDomain.value
}
