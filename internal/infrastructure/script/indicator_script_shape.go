package script

import (
	"fmt"
	"reflect"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptShape maps a declared value kind to the entry-point signature and result reading, keeping reflection out of the domain.
type indicatorScriptShape struct {
	resultType domains.IndicatorResultTypeDomain
	// inputSliceType is K candles for spot scripts and contract bars for contract ones.
	inputSliceType reflect.Type
}

func (indicatorScriptShape indicatorScriptShape) entryPointType() reflect.Type {
	if indicatorScriptShape.resultType.IsSignal() {
		return reflect.FuncOf(
			[]reflect.Type{indicatorScriptShape.inputSliceType},
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
		[]reflect.Type{indicatorScriptShape.inputSliceType},
		[]reflect.Type{reflect.MapOf(reflect.TypeOf(""), elementType)},
		false)
}

// readValues relies on the signature check for shape; an empty result is valid.
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

// readSignal rejects an unset (zero) signal as the script's mistake.
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

// valueOf stores a single value and a series alike.
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
