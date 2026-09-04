package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableStrategyParameterKinds is the entire set a caller may declare, in the
// order it is offered back when a declaration is not recognised.
//
// Adding another **numeric** kind means adding it here and answering the predicate
// below. Adding a kind that is not a number — a yes-or-no, say — needs more than
// this list: the value is one number, and widening it to hold anything would put the
// kind back at reading time, which is the one thing this design refuses.
var declarableStrategyParameterKinds = []vo.StrategyParameterKindVo{
	vo.StrategyParameterKindLookbackCount,
	vo.StrategyParameterKindNumber,
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
