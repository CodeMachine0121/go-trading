package script

import (
	"fmt"
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
// It asks the kind three questions — is the value a series, does it hold numbers, is
// it a signal — so supporting the four map-shaped kinds adds nothing here. The signal
// kind is the one genuinely different content shape: no indicator name, one value,
// and a value the script may only pick from a fixed set.
type indicatorScriptShape struct {
	resultType domains.IndicatorResultTypeDomain
}

// entryPointType is the exact form the entry point must have under this kind.
func (indicatorScriptShape indicatorScriptShape) entryPointType() reflect.Type {
	kCandleSliceType := reflect.TypeOf([]vo.KCandleVo(nil))

	if indicatorScriptShape.resultType.IsSignal() {
		return reflect.FuncOf(
			[]reflect.Type{kCandleSliceType},
			[]reflect.Type{reflect.TypeOf(vo.SignalVo(""))},
			false)
	}

	elementType := reflect.TypeOf(false)
	if indicatorScriptShape.resultType.HoldsNumbers() {
		elementType = reflect.TypeOf(float64(0))
	}

	if indicatorScriptShape.resultType.IsList() {
		elementType = reflect.SliceOf(elementType)
	}

	return reflect.FuncOf(
		[]reflect.Type{kCandleSliceType},
		[]reflect.Type{reflect.MapOf(reflect.TypeOf(""), elementType)},
		false)
}

// readValues collects what the entry point handed back. The form check has already
// guaranteed its shape, so one walk serves every map-shaped kind. A script that named
// nothing gives an empty set, which is a valid result rather than a failure.
func (indicatorScriptShape indicatorScriptShape) readValues(
	calculated reflect.Value,
) (map[string]vo.IndicatorValueVo, error) {
	if indicatorScriptShape.resultType.IsSignal() {
		return indicatorScriptShape.readSignal(calculated)
	}

	indicatorValues := map[string]vo.IndicatorValueVo{}

	calculatedValues := reflect.ValueOf(calculated.Interface())
	if calculatedValues.IsNil() {
		return indicatorValues, nil
	}

	valueIterator := calculatedValues.MapRange()
	for valueIterator.Next() {
		indicatorValues[valueIterator.Key().String()] =
			indicatorScriptShape.valueOf(valueIterator.Value())
	}

	return indicatorValues, nil
}

// readSignal reads the one signal a signal-kind script handed back. The form check
// has already guaranteed the return type is indicator.Signal; what is checked here is
// that the script actually set it to one of buy, sell or hold. A signal left unset —
// the zero value — is a script that built an opinion and never gave it a direction:
// the script's mistake to fix, not an opinion to bet money on.
func (indicatorScriptShape indicatorScriptShape) readSignal(
	calculated reflect.Value,
) (map[string]vo.IndicatorValueVo, error) {
	signal := vo.SignalVo(reflect.ValueOf(calculated.Interface()).String())

	switch signal {
	case vo.SignalBuy, vo.SignalSell, vo.SignalHold:
		return map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: signal}}, nil
	case "":
		return nil, fmt.Errorf(
			"%w: 算式產出了信號，但沒有設定方向——必須是 indicator.Buy、indicator.Sell 或 indicator.Hold 其中一個",
			domains.ErrIndicatorScriptFailed)
	default:
		return nil, fmt.Errorf(
			"%w: 認不得的信號 %q——必須是 indicator.Buy、indicator.Sell 或 indicator.Hold 其中一個",
			domains.ErrIndicatorScriptFailed, string(signal))
	}
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
