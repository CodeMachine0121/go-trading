package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableIndicatorResultTypes is in the order offered back on an unrecognised declaration; adding a kind only requires answering the predicates below.
var declarableIndicatorResultTypes = []vo.IndicatorResultTypeVo{
	vo.IndicatorResultTypeFloat,
	vo.IndicatorResultTypeFloatList,
	vo.IndicatorResultTypeBool,
	vo.IndicatorResultTypeBoolList,
	vo.IndicatorResultTypeSignal,
}

// IndicatorResultTypeDomain describes every kind via IsList, HoldsNumbers and IsSignal, so nothing downstream branches per kind; its zero value is unusable.
type IndicatorResultTypeDomain struct {
	value vo.IndicatorResultTypeVo
}

// NewIndicatorResultTypeDomain defaults a blank declaration to float for backward compatibility and matches case-insensitively; the refusal carries no sentinel because the caller decides its category.
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
		"指標值種類只能是 %s 其中之一", strings.Join(declarableSpellings, "、"))
}

func (indicatorResultTypeDomain IndicatorResultTypeDomain) Value() vo.IndicatorResultTypeVo {
	return indicatorResultTypeDomain.value
}

// IsList reports whether each indicator carries a series rather than a single value.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) IsList() bool {
	return indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloatList ||
		indicatorResultTypeDomain.value == vo.IndicatorResultTypeBoolList
}

// HoldsNumbers reports whether values are numbers rather than booleans.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) HoldsNumbers() bool {
	return indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloat ||
		indicatorResultTypeDomain.value == vo.IndicatorResultTypeFloatList
}

// IsSignal reports whether the whole result is one unnamed trading signal.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) IsSignal() bool {
	return indicatorResultTypeDomain.value == vo.IndicatorResultTypeSignal
}

// ScriptResultShape describes the expected Go result type, for mismatch error messages.
func (indicatorResultTypeDomain IndicatorResultTypeDomain) ScriptResultShape() string {
	if indicatorResultTypeDomain.IsSignal() {
		return "indicator.Signal"
	}

	elementShape := "bool"
	if indicatorResultTypeDomain.HoldsNumbers() {
		elementShape = "float64"
	}

	if indicatorResultTypeDomain.IsList() {
		elementShape = "[]" + elementShape
	}

	return "map[string]" + elementShape
}
