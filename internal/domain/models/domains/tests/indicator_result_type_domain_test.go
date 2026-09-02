package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIndicatorResultTypeDomainReadsWhatWasDeclared(t *testing.T) {
	testCases := []struct {
		name                 string
		declared             string
		expectedValue        vo.IndicatorResultTypeVo
		expectedIsList       bool
		expectedHoldsNumbers bool
	}{
		{
			name: "one number", declared: "float", expectedValue: vo.IndicatorResultTypeFloat,
			expectedIsList: false, expectedHoldsNumbers: true,
		},
		{
			name: "a series of numbers", declared: "floatList", expectedValue: vo.IndicatorResultTypeFloatList,
			expectedIsList: true, expectedHoldsNumbers: true,
		},
		{
			name: "one answer", declared: "bool", expectedValue: vo.IndicatorResultTypeBool,
			expectedIsList: false, expectedHoldsNumbers: false,
		},
		{
			name: "a series of answers", declared: "boolList", expectedValue: vo.IndicatorResultTypeBoolList,
			expectedIsList: true, expectedHoldsNumbers: false,
		},
		{
			name: "nothing declared falls back to one number", declared: "",
			expectedValue: vo.IndicatorResultTypeFloat, expectedIsList: false, expectedHoldsNumbers: true,
		},
		{
			name: "surrounding blanks and letter case are forgiven", declared: "  FloatList ",
			expectedValue: vo.IndicatorResultTypeFloatList, expectedIsList: true, expectedHoldsNumbers: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resultType, err := domains.NewIndicatorResultTypeDomain(testCase.declared)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedValue, resultType.Value())
			assert.Equal(t, testCase.expectedIsList, resultType.IsList())
			assert.Equal(t, testCase.expectedHoldsNumbers, resultType.HoldsNumbers())
		})
	}
}

func TestNewIndicatorResultTypeDomainRefusesAnythingElse(t *testing.T) {
	for _, declared := range []string{"string", "int", "float64", "[]float64", "list"} {
		t.Run(declared, func(t *testing.T) {
			_, err := domains.NewIndicatorResultTypeDomain(declared)

			assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, err.Error(), "指標值種類只能是")
			assert.Contains(t, err.Error(), "float、floatList、bool、boolList")
		})
	}
}

func TestScriptResultShapeSpellsOutWhatAScriptMustHandBack(t *testing.T) {
	testCases := []struct {
		declared      string
		expectedShape string
	}{
		{declared: "float", expectedShape: "map[string]float64"},
		{declared: "floatList", expectedShape: "map[string][]float64"},
		{declared: "bool", expectedShape: "map[string]bool"},
		{declared: "boolList", expectedShape: "map[string][]bool"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.declared, func(t *testing.T) {
			resultType, err := domains.NewIndicatorResultTypeDomain(testCase.declared)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedShape, resultType.ScriptResultShape())
		})
	}
}
