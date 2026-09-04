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
	parameters domains.StrategyParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	inputKCandles := kCandles
	// The reader records the first knob a script reaches for that nobody declared.
	// It is consulted before the error the script came back with, so the answer does
	// not depend on how the interpreter happens to word a panic — that is somebody
	// else's implementation detail and it changes between versions.
	parameterReader := &strategyParameterReader{parameters: parameters}
	scriptSymbols := interp.Exports{
		scriptDataPackage: {
			"KCandle":       reflect.ValueOf((*vo.KCandleVo)(nil)),
			"Data":          reflect.ValueOf(&inputKCandles).Elem(),
			"LookbackCount": reflect.ValueOf(parameterReader.lookbackCount),
			"Number":        reflect.ValueOf(parameterReader.number),
			"Boolean":       reflect.ValueOf(parameterReader.boolean),
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

	// Asked before anything else, because a script that reached for a knob nobody
	// declared did not fail on its own terms — it failed because a name does not
	// match, and every other answer here would send the reader to the wrong place.
	if missingName, isMissing := parameterReader.missingName(); isMissing {
		return nil, domains.UndeclaredParameter(missingName)
	}

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

// strategyParameterReader is what a script reaches a knob through.
//
// A name nobody declared is recorded and then panicked on rather than answered with
// a zero. A zero looks like an answer: a loop reaching back zero candles still
// produces a list of numbers, and somebody would act on it. Worse, a zero can turn
// a bounded loop into one that runs until the whole allowance is spent, so the
// failure arrives late as well as wrong.
//
// The panic is caught by the interpreter and comes back as an ordinary error; what
// makes the report correct is the recorded name, not the panic.
type strategyParameterReader struct {
	parameters domains.StrategyParametersDomain
	missing    string
	hasMissing bool
}

func (reader *strategyParameterReader) lookbackCount(name string) int {
	lookbackCount, isDeclared := reader.parameters.LookbackCountOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return lookbackCount
}

func (reader *strategyParameterReader) number(name string) float64 {
	number, isDeclared := reader.parameters.NumberOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return number
}

func (reader *strategyParameterReader) boolean(name string) bool {
	isTrue, isDeclared := reader.parameters.BooleanOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return isTrue
}

// recordMissing keeps the first name that did not match. The first is the one worth
// reporting: the ones after it are usually the same mistake spreading, and a list of
// them would bury the one line the reader has to go and fix.
func (reader *strategyParameterReader) recordMissing(name string) {
	if !reader.hasMissing {
		reader.missing = name
		reader.hasMissing = true
	}

	panic(errParameterNotDeclared)
}

func (reader *strategyParameterReader) missingName() (string, bool) {
	return reader.missing, reader.hasMissing
}

// errParameterNotDeclared is what the reader panics with. Nothing matches on it —
// the recorded name is what the report is built from — but panicking with a value of
// its own keeps this apart from a panic the script itself caused.
var errParameterNotDeclared = errors.New("indicator parameter not declared")
