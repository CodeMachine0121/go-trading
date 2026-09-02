package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableIndicatorResultTypes is the entire set a caller may declare, in the order
// it is offered back when a declaration is not recognised. Supporting one more kind
// means adding it here and answering the two predicates below for it — the script
// runner never learns a new branch.
var declarableIndicatorResultTypes = []vo.IndicatorResultTypeVo{
	vo.IndicatorResultTypeFloat,
	vo.IndicatorResultTypeFloatList,
	vo.IndicatorResultTypeBool,
	vo.IndicatorResultTypeBoolList,
}

// IndicatorResultTypeDomain is a declared indicator value kind and everything the
// rest of the system needs to know about it. It answers two questions — is the value
// a series, and does it hold numbers — and those two answers are enough to describe
// every kind, which is why nothing downstream branches per kind.
//
// Its zero value is not a usable kind; it is only ever returned alongside an error.
type IndicatorResultTypeDomain struct {
	value vo.IndicatorResultTypeVo
}

// NewIndicatorResultTypeDomain reads what the caller declared. Declaring nothing means
// one number per indicator, which is what every caller got before kinds existed, so
// requests written against the older behavior keep working untouched. Spelling is
// forgiving about surrounding blanks and letter case; anything else is refused, naming
// what could have been declared instead.
func NewIndicatorResultTypeDomain(declared string) (IndicatorResultTypeDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return IndicatorResultTypeDomain{value: vo.IndicatorResultTypeFloat}, nil
	}

	for _, declarableResultType := range declarableIndicatorResultTypes {
		if strings.EqualFold(string(declarableResultType), normalizedDeclaration) {
			return IndicatorResultTypeDomain{value: declarableResultType}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableIndicatorResultTypes))
	for _, declarableResultType := range declarableIndicatorResultTypes {
		declarableSpellings = append(declarableSpellings, string(declarableResultType))
	}

	return IndicatorResultTypeDomain{}, fmt.Errorf(
		"%w: 指標值種類只能是 %s 其中之一",
		ErrIndicatorCalculationValidation, strings.Join(declarableSpellings, "、"))
}

func (indicatorResultTypeDomain IndicatorResultTypeDomain) Value() vo.IndicatorResultTypeVo {
	return indicatorResultTypeDomain.value
}

// IsList says whether each indicator carries a series rather than a lone value.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) IsList() bool {
	return indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloatList ||
		indicatorResultTypeDomain.value == vo.IndicatorResultTypeBoolList
}

// HoldsNumbers says whether the content is numbers rather than true/false answers.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) HoldsNumbers() bool {
	return indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloat ||
		indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloatList
}

// ScriptResultShape spells out what a script must hand back under this kind, so that a
// script whose shape does not match can be told what was expected of it.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) ScriptResultShape() string {
	elementShape := "bool"
	if indicatorResultTypeDomain.HoldsNumbers() {
		elementShape = "float64"
	}

	if indicatorResultTypeDomain.IsList() {
		elementShape = "[]" + elementShape
	}

	return "map[string]" + elementShape
}
