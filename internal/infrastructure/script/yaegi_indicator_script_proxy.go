package script

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// errScriptAllowanceSpent is why a run was given up on when the script outlived its
// allowance. It is carried as the deadline's cause so that this reason can be told
// apart from the caller having gone away, which reaches the same context but is not
// the script's fault and must not be reported as though it were.
var errScriptAllowanceSpent = errors.New("indicator script allowance spent")

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
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
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

	scriptShape := indicatorScriptShape{resultType: resultType}
	if entryPoint.Type() != scriptShape.entryPointType() {
		return nil, fmt.Errorf(
			"%w: 宣告的指標值種類是 %s，Calculate 的形式必須是 func Calculate(data []indicator.KCandle) %s",
			domains.ErrIndicatorScriptFailed, resultType.Value(), resultType.ScriptResultShape())
	}

	// The allowance is measured from the caller's own context rather than from a
	// fresh one, so a caller that has gone away takes its script with it instead of
	// leaving it running for the rest of the allowance with nobody left to answer.
	allowanceContext, stopWaiting := context.WithTimeoutCause(
		executionContext, yaegiIndicatorScriptProxy.executionTimeout, errScriptAllowanceSpent)
	defer stopWaiting()

	calculated, callError := interpreter.EvalWithContext(allowanceContext, scriptCall)
	if errors.Is(context.Cause(allowanceContext), errScriptAllowanceSpent) {
		return nil, fmt.Errorf(
			"%w: 算式在 %s 內未能算完，已中止",
			domains.ErrIndicatorScriptFailed, yaegiIndicatorScriptProxy.executionTimeout)
	}
	if allowanceContext.Err() != nil {
		return nil, fmt.Errorf(
			"%w: 算式已中止，因為發動它的請求已經結束", domains.ErrIndicatorScriptFailed)
	}
	if callError != nil {
		return nil, fmt.Errorf(
			"%w: 算式執行失敗：%v", domains.ErrIndicatorScriptFailed, callError)
	}

	return scriptShape.readValues(calculated), nil
}
