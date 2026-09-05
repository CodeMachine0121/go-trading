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
// as a script failure with no partial result.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) Execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
	parameters domains.StrategyParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	preparedScript, prepareError := yaegiIndicatorScriptProxy.prepare(script, resultType, parameters)
	if prepareError != nil {
		return nil, prepareError
	}

	return preparedScript.runOver(executionContext, kCandles)
}

// ExecuteForEachCandle runs the same script once per K candle: the nth run sees the
// candles from the first up to and including the nth, and the results come back in
// that same order, one set per candle.
//
// It exists here rather than as a loop in the caller because only this side can do the
// thing that makes it affordable: read the script **once** and then feed it different
// data. A caller looping over Execute would rebuild the interpreter and re-read the
// whole script on every candle — a fixed cost paid a thousand times over for a replay
// of a thousand candles.
//
// Every candle's run gets the full allowance of its own, and the first failure ends
// everything with no partial result: half a replay is not a shorter replay, it is a
// wrong one.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) ExecuteForEachCandle(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
	parameters domains.StrategyParametersDomain,
) ([]map[string]vo.IndicatorValueVo, error) {
	preparedScript, prepareError := yaegiIndicatorScriptProxy.prepare(script, resultType, parameters)
	if prepareError != nil {
		return nil, prepareError
	}

	perCandleIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(kCandles))
	for candleCount := 1; candleCount <= len(kCandles); candleCount++ {
		indicatorValues, executionError := preparedScript.runOver(
			executionContext, kCandles[:candleCount])
		if executionError != nil {
			return nil, executionError
		}

		perCandleIndicatorValues = append(perCandleIndicatorValues, indicatorValues)
	}

	return perCandleIndicatorValues, nil
}

// prepare reads the script once: it builds the interpreter, hands it the only world a
// script may reach, evaluates the source, and checks that the entry point has the
// exact form the declared kind calls for.
//
// Everything that can be judged without data is judged here, so a script that is
// broken is reported as broken before a single candle has been looked at — rather than
// on the first candle, which would read like the data's fault.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) prepare(
	script string,
	resultType domains.IndicatorResultTypeDomain,
	parameters domains.StrategyParametersDomain,
) (*preparedScript, error) {
	// The reader records the first knob a script reaches for that nobody declared.
	// It is consulted before the error the script came back with, so the answer does
	// not depend on how the interpreter happens to word a panic — that is somebody
	// else's implementation detail and it changes between versions.
	preparedScript := &preparedScript{
		interpreter:      interp.New(interp.Options{}),
		shape:            indicatorScriptShape{resultType: resultType},
		parameterReader:  &strategyParameterReader{parameters: parameters},
		executionTimeout: yaegiIndicatorScriptProxy.executionTimeout,
	}

	scriptSymbols := interp.Exports{
		scriptDataPackage: {
			"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
			// The interpreter is handed the address of the field rather than a copy of
			// its value, so that what a script sees can be replaced between runs. That
			// is the whole mechanism behind reading the script once and replaying it
			// over a growing stretch of market.
			"Data":          reflect.ValueOf(&preparedScript.visibleKCandles).Elem(),
			"LookbackCount": reflect.ValueOf(preparedScript.parameterReader.lookbackCount),
			"Number":        reflect.ValueOf(preparedScript.parameterReader.number),
			"Boolean":       reflect.ValueOf(preparedScript.parameterReader.boolean),
		},
	}
	for packagePath, packageSymbols := range allowedPackages {
		scriptSymbols[packagePath] = packageSymbols
	}

	// The symbol table is assembled here from compile-time constants, so this cannot
	// fail; were it ever malformed, the script would simply fail to read on the next
	// line and be reported the same way as any other unreadable script.
	_ = preparedScript.interpreter.Use(scriptSymbols)

	if _, evalError := preparedScript.interpreter.Eval(script); evalError != nil {
		return nil, fmt.Errorf(
			"%w: 算式無法解讀：%v", domains.ErrIndicatorScriptFailed, evalError)
	}

	entryPoint, lookupError := preparedScript.interpreter.Eval(scriptEntryPoint)
	if lookupError != nil {
		return nil, fmt.Errorf(
			"%w: 算式必須提供 Calculate 進入點：%v", domains.ErrIndicatorScriptFailed, lookupError)
	}

	if entryPoint.Type() != preparedScript.shape.entryPointType() {
		return nil, fmt.Errorf(
			"%w: 宣告的指標值種類是 %s，Calculate 的形式必須是 func Calculate(data []indicator.KCandle) %s",
			domains.ErrIndicatorScriptFailed, resultType.Value(), resultType.ScriptResultShape())
	}

	return preparedScript, nil
}

// preparedScript is one script that has already been read and accepted, waiting to be
// run over some candles. Holding the interpreter open between runs is what makes
// replaying a strategy affordable; holding the candles in a field is what makes those
// runs see different data.
type preparedScript struct {
	interpreter *interp.Interpreter
	shape       indicatorScriptShape
	// visibleKCandles is what the script sees. Its address is in the interpreter's
	// symbol table, so assigning to it changes the script's input without re-reading
	// a single line of the script.
	visibleKCandles  []vo.KCandleVo
	parameterReader  *strategyParameterReader
	executionTimeout time.Duration
}

// runOver runs the already-read script over exactly these candles. Running the entry
// point through the interpreter rather than calling it directly is what makes both the
// giving up and the reporting possible: the interpreter turns every failure, including
// a deliberate one, into an error rather than letting it escape.
func (preparedScript *preparedScript) runOver(
	executionContext context.Context, kCandles []vo.KCandleVo,
) (map[string]vo.IndicatorValueVo, error) {
	preparedScript.visibleKCandles = kCandles

	// The allowance is measured from the caller's own context rather than from a
	// fresh one, so a caller that has gone away takes its script with it instead of
	// leaving it running for the rest of the allowance with nobody left to answer.
	allowanceContext, stopWaiting := context.WithTimeoutCause(
		executionContext, preparedScript.executionTimeout, errScriptAllowanceSpent)
	defer stopWaiting()

	calculated, callError := preparedScript.interpreter.EvalWithContext(allowanceContext, scriptCall)

	// Asked before anything else, because a script that reached for a knob nobody
	// declared did not fail on its own terms — it failed because a name does not
	// match, and every other answer here would send the reader to the wrong place.
	if missingName, isMissing := preparedScript.parameterReader.missingName(); isMissing {
		return nil, domains.UndeclaredParameter(missingName)
	}

	if errors.Is(context.Cause(allowanceContext), errScriptAllowanceSpent) {
		return nil, fmt.Errorf(
			"%w: 算式在 %s 內未能算完，已中止",
			domains.ErrIndicatorScriptFailed, preparedScript.executionTimeout)
	}
	if allowanceContext.Err() != nil {
		return nil, fmt.Errorf(
			"%w: 算式已中止，因為發動它的請求已經結束", domains.ErrIndicatorScriptFailed)
	}
	if callError != nil {
		return nil, fmt.Errorf(
			"%w: 算式執行失敗：%v", domains.ErrIndicatorScriptFailed, callError)
	}

	return preparedScript.shape.readValues(calculated), nil
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
