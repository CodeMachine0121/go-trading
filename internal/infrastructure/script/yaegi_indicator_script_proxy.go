package script

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// scriptEntryPoint is the one name every indicator script must define.
const scriptEntryPoint = "main.Calculate"

// scriptCall is how that entry point is invoked. Running it through the interpreter
// rather than calling it directly is what lets a script that never finishes be
// stopped when its time runs out.
const scriptCall = "main.Calculate(indicator.Data)"

// scriptDataPackage is the package a script imports to reach its input.
const scriptDataPackage = "indicator/indicator"

// allowedPackages is the entire world an indicator script can reach. Anything not
// listed here cannot be imported, which is what keeps a script to pure arithmetic:
// no files, no network, no clock, no randomness. Widening what scripts may do
// means adding an entry here and nowhere else.
var allowedPackages = interp.Exports{
	"math/math": stdlib.Symbols["math/math"],
	"sort/sort": stdlib.Symbols["sort/sort"],
}

// YaegiIndicatorScriptProxy runs indicator scripts with an embedded Go interpreter,
// giving up on any script that outlives its allowance.
type YaegiIndicatorScriptProxy struct {
	executionTimeout time.Duration
}

func NewYaegiIndicatorScriptProxy(executionTimeout time.Duration) *YaegiIndicatorScriptProxy {
	return &YaegiIndicatorScriptProxy{executionTimeout: executionTimeout}
}

// Execute runs the script over the K candles and collects its values in the declared
// kind. Anything that goes wrong — the script cannot be read, it has no usable entry
// point, it hands back a shape other than the declared one, it reaches for something
// it may not use, it fails while running, or it outlives its allowance — is reported
// as a script failure with no partial result. Running the entry point through the
// interpreter rather than calling it directly is what makes both the giving up and
// the reporting possible: the interpreter turns every failure, including a deliberate
// one, into an error rather than letting it escape.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) Execute(
	script string, resultType domains.IndicatorResultTypeDomain, kCandles []vo.KCandleVo,
) (map[string]vo.IndicatorValueVo, error) {
	inputKCandles := kCandles
	scriptSymbols := interp.Exports{
		scriptDataPackage: {
			"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
			"Data":    reflect.ValueOf(&inputKCandles).Elem(),
		},
	}
	for packagePath, packageSymbols := range allowedPackages {
		scriptSymbols[packagePath] = packageSymbols
	}

	interpreter := interp.New(interp.Options{})

	// The symbol table is assembled here from compile-time constants, so this cannot
	// fail; were it ever malformed, the script would simply fail to read on the next
	// line and be reported the same way as any other unreadable script.
	_ = interpreter.Use(scriptSymbols)

	if _, evalError := interpreter.Eval(script); evalError != nil {
		return nil, fmt.Errorf(
			"%w: 算式無法解讀：%v", domains.ErrIndicatorScriptFailed, evalError)
	}

	entryPoint, lookupError := interpreter.Eval(scriptEntryPoint)
	if lookupError != nil {
		return nil, fmt.Errorf(
			"%w: 算式必須提供 Calculate 進入點：%v", domains.ErrIndicatorScriptFailed, lookupError)
	}

	if entryPoint.Type() != entryPointTypeFor(resultType) {
		return nil, fmt.Errorf(
			"%w: 宣告的指標值種類是 %s，Calculate 的形式必須是 func Calculate(data []indicator.KCandle) %s",
			domains.ErrIndicatorScriptFailed, resultType.Value(), resultType.ScriptResultShape())
	}

	executionContext, stopWaiting := context.WithTimeout(context.Background(), yaegiIndicatorScriptProxy.executionTimeout)
	defer stopWaiting()

	calculated, callError := interpreter.EvalWithContext(executionContext, scriptCall)
	if executionContext.Err() != nil {
		return nil, fmt.Errorf(
			"%w: 算式在 %s 內未能算完，已中止",
			domains.ErrIndicatorScriptFailed, yaegiIndicatorScriptProxy.executionTimeout)
	}
	if callError != nil {
		return nil, fmt.Errorf(
			"%w: 算式執行失敗：%v", domains.ErrIndicatorScriptFailed, callError)
	}

	return readIndicatorValues(resultType, calculated), nil
}

// entryPointTypeFor is the exact form the entry point must have under this kind. It
// is built from the kind's two predicates rather than picked from a list of four, so
// supporting one more kind never adds a branch here.
func entryPointTypeFor(resultType domains.IndicatorResultTypeDomain) reflect.Type {
	elementType := reflect.TypeOf(false)
	if resultType.HoldsNumbers() {
		elementType = reflect.TypeOf(float64(0))
	}

	if resultType.IsList() {
		elementType = reflect.SliceOf(elementType)
	}

	return reflect.FuncOf(
		[]reflect.Type{reflect.TypeOf([]vo.KCandleVo(nil))},
		[]reflect.Type{reflect.MapOf(reflect.TypeOf(""), elementType)},
		false)
}

// readIndicatorValues collects what the entry point handed back. Its shape is already
// guaranteed by the form check above, so this walk is the same for every kind: the
// predicates say where the content goes and whether it is a series. A script that
// named nothing gives an empty set, which is a valid result rather than a failure.
func readIndicatorValues(
	resultType domains.IndicatorResultTypeDomain, calculated reflect.Value,
) map[string]vo.IndicatorValueVo {
	indicatorValues := map[string]vo.IndicatorValueVo{}

	calculatedValues := reflect.ValueOf(calculated.Interface())
	if calculatedValues.IsNil() {
		return indicatorValues
	}

	valueIterator := calculatedValues.MapRange()
	for valueIterator.Next() {
		indicatorValues[valueIterator.Key().String()] = indicatorValueOf(resultType, valueIterator.Value())
	}

	return indicatorValues
}

// indicatorValueOf reads one named value. A lone value and a series are stored alike —
// a series is simply the elements it holds — so only the kind's two predicates decide
// what is read and where it is put.
func indicatorValueOf(
	resultType domains.IndicatorResultTypeDomain, calculatedValue reflect.Value,
) vo.IndicatorValueVo {
	elements := []reflect.Value{calculatedValue}
	if resultType.IsList() {
		elements = make([]reflect.Value, 0, calculatedValue.Len())
		for elementIndex := 0; elementIndex < calculatedValue.Len(); elementIndex++ {
			elements = append(elements, calculatedValue.Index(elementIndex))
		}
	}

	if !resultType.HoldsNumbers() {
		booleans := make([]bool, 0, len(elements))
		for _, element := range elements {
			booleans = append(booleans, element.Bool())
		}

		return vo.IndicatorValueVo{IsList: resultType.IsList(), Booleans: booleans}
	}

	numbers := make([]float64, 0, len(elements))
	for _, element := range elements {
		numbers = append(numbers, element.Float())
	}

	return vo.IndicatorValueVo{IsList: resultType.IsList(), Numbers: numbers}
}
