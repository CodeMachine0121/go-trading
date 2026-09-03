package script

import (
	"reflect"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptShape is what a declared indicator value kind looks like to the
// interpreter: the form a script's entry point must have, and how to read what that
// entry point hands back. It is the only place the running of a script meets the
// kind that was declared, and the only place that needs the Go runtime's own view of
// types — which is why the kind's own model, living in the domain, stays free of it.
//
// It never asks "which kind is this". Every kind is described by the same two
// questions — is the value a series, and does it hold numbers — so supporting one
// more kind adds nothing here.
type indicatorScriptShape struct {
	resultType domains.IndicatorResultTypeDomain
}

// entryPointType is the exact form the entry point must have under this kind.
func (indicatorScriptShape indicatorScriptShape) entryPointType() reflect.Type {
	elementType := reflect.TypeOf(false)
	if indicatorScriptShape.resultType.HoldsNumbers() {
		elementType = reflect.TypeOf(float64(0))
	}

	if indicatorScriptShape.resultType.IsList() {
		elementType = reflect.SliceOf(elementType)
	}

	return reflect.FuncOf(
		[]reflect.Type{reflect.TypeOf([]vo.KCandleVo(nil))},
		[]reflect.Type{reflect.MapOf(reflect.TypeOf(""), elementType)},
		false)
}

// readValues collects what the entry point handed back. The form check has already
// guaranteed its shape, so one walk serves every kind. A script that named nothing
// gives an empty set, which is a valid result rather than a failure.
func (indicatorScriptShape indicatorScriptShape) readValues(
	calculated reflect.Value,
) map[string]vo.IndicatorValueVo {
	indicatorValues := map[string]vo.IndicatorValueVo{}

	calculatedValues := reflect.ValueOf(calculated.Interface())
	if calculatedValues.IsNil() {
		return indicatorValues
	}

	valueIterator := calculatedValues.MapRange()
	for valueIterator.Next() {
		indicatorValues[valueIterator.Key().String()] =
			indicatorScriptShape.valueOf(valueIterator.Value())
	}

	return indicatorValues
}

// valueOf reads one named value. A lone value and a series are stored alike — a
// series is simply the elements it holds — so only the two questions above decide
// what is read and where it is put.
func (indicatorScriptShape indicatorScriptShape) valueOf(
	calculatedValue reflect.Value,
) vo.IndicatorValueVo {
	elements := []reflect.Value{calculatedValue}
	if indicatorScriptShape.resultType.IsList() {
		elements = make([]reflect.Value, 0, calculatedValue.Len())
		for elementIndex := range calculatedValue.Len() {
			elements = append(elements, calculatedValue.Index(elementIndex))
		}
	}

	if !indicatorScriptShape.resultType.HoldsNumbers() {
		booleans := make([]bool, 0, len(elements))
		for _, element := range elements {
			booleans = append(booleans, element.Bool())
		}

		return vo.IndicatorValueVo{IsList: indicatorScriptShape.resultType.IsList(), Booleans: booleans}
	}

	numbers := make([]float64, 0, len(elements))
	for _, element := range elements {
		numbers = append(numbers, element.Float())
	}

	return vo.IndicatorValueVo{IsList: indicatorScriptShape.resultType.IsList(), Numbers: numbers}
}
