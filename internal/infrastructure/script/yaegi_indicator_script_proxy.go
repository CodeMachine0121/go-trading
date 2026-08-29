package script

import (
	"fmt"
	"reflect"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// scriptEntryPoint is the one name every indicator script must define.
const scriptEntryPoint = "main.Calculate"

// allowedSymbols is the entire world an indicator script can reach. Anything not
// listed here cannot be imported, which is what keeps a script to pure arithmetic:
// no files, no network, no clock, no randomness. Widening what scripts may do
// means adding an entry here and nowhere else.
var allowedSymbols = interp.Exports{
	"math/math": stdlib.Symbols["math/math"],
	"indicator/indicator": {
		"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
	},
}

// YaegiIndicatorScriptProxy runs indicator scripts with an embedded Go interpreter.
type YaegiIndicatorScriptProxy struct{}

func NewYaegiIndicatorScriptProxy() *YaegiIndicatorScriptProxy {
	return &YaegiIndicatorScriptProxy{}
}

// Execute runs the script over the K candles. Anything that goes wrong — the script
// cannot be read, it has no usable entry point, it reaches for something it may not
// use, or it fails while running — is reported as a script failure with no partial
// result.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) Execute(
	script string, kCandles []vo.KCandleVo,
) (indicatorValues map[string]float64, executionError error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			indicatorValues = nil
			executionError = fmt.Errorf(
				"%w: 算式執行失敗：%v", domains.ErrIndicatorScriptFailed, recovered)
		}
	}()

	interpreter := interp.New(interp.Options{})

	// The symbol table is a compile-time constant, so this cannot fail here; were it
	// ever malformed, the script would simply fail to read on the next line and be
	// reported the same way as any other unreadable script.
	_ = interpreter.Use(allowedSymbols)

	if _, evalError := interpreter.Eval(script); evalError != nil {
		return nil, fmt.Errorf(
			"%w: 算式無法解讀：%v", domains.ErrIndicatorScriptFailed, evalError)
	}

	entryPoint, lookupError := interpreter.Eval(scriptEntryPoint)
	if lookupError != nil {
		return nil, fmt.Errorf(
			"%w: 算式必須提供 Calculate 進入點：%v", domains.ErrIndicatorScriptFailed, lookupError)
	}

	calculate, hasExpectedShape := entryPoint.Interface().(func([]vo.KCandleVo) map[string]float64)
	if !hasExpectedShape {
		return nil, fmt.Errorf(
			"%w: Calculate 的形式必須是 func Calculate(data []indicator.KCandle) map[string]float64",
			domains.ErrIndicatorScriptFailed)
	}

	calculatedValues := calculate(kCandles)
	if calculatedValues == nil {
		return map[string]float64{}, nil
	}

	return calculatedValues, nil
}
