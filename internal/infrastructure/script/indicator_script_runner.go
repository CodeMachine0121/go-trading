package script

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

// indicatorScriptRunner is everything about running an indicator script over one
// kind of input: the sandbox, the symbols a script may reach, the check that its entry
// point has the form the declared kind calls for, the allowance, and the replay over a
// growing stretch of market.
//
// It is generic over the element a script is fed because that element is the one
// thing that differs between the markets a script can eat. Spot scripts are fed K
// candles and contract scripts are fed contract bars, and apart from the entry point's
// argument and the types a script can name, running the two is the same work — work
// that must not drift apart, since a fix to the allowance or to how a failure is
// reported belongs to both.
type indicatorScriptRunner[Input any] struct {
	executionTimeout time.Duration
	// inputTypeName is how a script names the element it is fed, as in
	// func Calculate(data []indicator.<inputTypeName>). It is only ever read to tell
	// the author what their entry point should look like.
	inputTypeName string
	// inputTypes are the types this kind of input lets a script name, keyed by the
	// name it names them by.
	inputTypes map[string]reflect.Value
}

// execute runs the script over the input once. Anything that goes wrong — the script
// cannot be read, it has no usable entry point, it hands back a shape other than the
// declared one, it reaches for something it may not use, it fails while running, or
// it outlives its allowance — is reported as a script failure with no partial result.
func (runner indicatorScriptRunner[Input]) execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	input []Input,
	parameters domains.StrategyScriptParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	preparedScript, prepareError := runner.prepare(script, resultType, parameters)
	if prepareError != nil {
		return nil, prepareError
	}

	// A copy of exactly the input's length, for the same two reasons the replay makes
	// one: nothing the script writes reaches the caller, and nothing past the input is
	// within its reach.
	isolatedInput := make([]Input, len(input))
	copy(isolatedInput, input)

	return preparedScript.runOver(executionContext, isolatedInput)
}

// executeForEachElement runs the same script once per element: the nth run sees the
// elements from the first up to and including the nth, and the results come back in
// that same order, one set per element.
//
// It lives here rather than as a loop in the caller because only this side can do the
// thing that makes it affordable: read the script **once** and then feed it different
// data. A caller looping over execute would rebuild the interpreter and re-read the
// whole script on every element — a fixed cost paid a thousand times over for a replay
// of a thousand candles.
//
// Every element's run gets the full allowance of its own, and the first failure ends
// everything with no partial result: half a replay is not a shorter replay, it is a
// wrong one.
func (runner indicatorScriptRunner[Input]) executeForEachElement(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	input []Input,
	parameters domains.StrategyScriptParametersDomain,
) ([]map[string]vo.IndicatorValueVo, error) {
	preparedScript, prepareError := runner.prepare(script, resultType, parameters)
	if prepareError != nil {
		return nil, prepareError
	}

	// The replay works on a copy of its own, so a script that writes into what it is
	// shown can never reach back into the caller's market. Each run is then cut to a
	// capacity equal to its length: a slice carries the array behind it, and without
	// that cut a script could re-slice up to cap(data) and read — or rewrite — the
	// candles it has not reached yet, which would make every replay a lie.
	isolatedInput := make([]Input, len(input))
	copy(isolatedInput, input)

	perElementIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(input))
	for elementCount := 1; elementCount <= len(input); elementCount++ {
		indicatorValues, executionError := preparedScript.runOver(
			executionContext, isolatedInput[:elementCount:elementCount])
		if executionError != nil {
			return nil, executionError
		}

		perElementIndicatorValues = append(perElementIndicatorValues, indicatorValues)
	}

	return perElementIndicatorValues, nil
}

// prepare reads the script once: it builds the interpreter, hands it the only world a
// script may reach, evaluates the source, and checks that the entry point has the
// exact form the declared kind calls for.
//
// Everything that can be judged without data is judged here, so a script that is
// broken is reported as broken before a single element has been looked at — rather
// than on the first one, which would read like the data's fault.
func (runner indicatorScriptRunner[Input]) prepare(
	script string,
	resultType domains.IndicatorResultTypeDomain,
	parameters domains.StrategyScriptParametersDomain,
) (*preparedScript[Input], error) {
	// The reader records the first knob a script reaches for that nobody declared.
	// It is consulted before the error the script came back with, so the answer does
	// not depend on how the interpreter happens to word a panic — that is somebody
	// else's implementation detail and it changes between versions.
	preparedScript := &preparedScript[Input]{
		interpreter: interp.New(interp.Options{}),
		shape: indicatorScriptShape{
			resultType:     resultType,
			inputSliceType: reflect.TypeOf([]Input(nil)),
		},
		parameterReader:  &strategyScriptParameterReader{parameters: parameters},
		executionTimeout: runner.executionTimeout,
	}

	dataSymbols := map[string]reflect.Value{
		// The interpreter is handed the address of the field rather than a copy of
		// its value, so that what a script sees can be replaced between runs. That
		// is the whole mechanism behind reading the script once and replaying it
		// over a growing stretch of market.
		"Data":          reflect.ValueOf(&preparedScript.visibleInput).Elem(),
		"LookbackCount": reflect.ValueOf(preparedScript.parameterReader.lookbackCount),
		"Number":        reflect.ValueOf(preparedScript.parameterReader.number),
		"Boolean":       reflect.ValueOf(preparedScript.parameterReader.boolean),
		// The only way a signal-kind script may state a signal: pick one of these
		// three. It cannot build its own — Signal is exported as a bare type with
		// no constructor — and it cannot name a fourth. These sit in scope for
		// every script; a script of another kind simply never returns one.
		"Signal": reflect.ValueOf((*vo.SignalVo)(nil)),
		"Buy":    reflect.ValueOf(vo.SignalBuy),
		"Sell":   reflect.ValueOf(vo.SignalSell),
		"Hold":   reflect.ValueOf(vo.SignalHold),
	}
	for typeName, typeSymbol := range runner.inputTypes {
		dataSymbols[typeName] = typeSymbol
	}

	scriptSymbols := interp.Exports{scriptDataPackage: dataSymbols}
	for packagePath, packageSymbols := range allowedPackages {
		scriptSymbols[packagePath] = packageSymbols
	}

	// The symbol table is assembled here from compile-time constants, so this cannot
	// fail; were it ever malformed, the script would simply fail to read on the next
	// line and be reported the same way as any other unreadable script.
	_ = preparedScript.interpreter.Use(scriptSymbols)

	// Goroutines and channels are refused before the script is evaluated. A goroutine
	// is the one thing a script can start that outlives the run: the allowance cannot
	// stop it, and a panic on it is beyond what the interpreter can catch, so it takes
	// the whole server down instead of failing the script. A channel is only good for
	// talking to a goroutine; without one, all it can do is block a run forever, and a
	// run blocked on a receive stays parked after its allowance is spent. Nothing an
	// indicator computes needs either. Source that does not parse is left to the
	// interpreter, which reports it the same way as any other unreadable script.
	if parsedScript, parseError := parser.ParseFile(token.NewFileSet(), "", script, 0); parseError == nil {
		reachesForConcurrency := false
		ast.Inspect(parsedScript, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.GoStmt, *ast.ChanType:
				reachesForConcurrency = true
			}
			return !reachesForConcurrency
		})
		if reachesForConcurrency {
			return nil, fmt.Errorf(
				"%w: 算式不得使用 goroutine（go 敘述）或 channel", domains.ErrIndicatorScriptFailed)
		}
	}

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
			"%w: 宣告的指標值種類是 %s，Calculate 的形式必須是 func Calculate(data []indicator.%s) %s",
			domains.ErrIndicatorScriptFailed, resultType.Value(), runner.inputTypeName,
			resultType.ScriptResultShape())
	}

	return preparedScript, nil
}

// preparedScript is one script that has already been read and accepted, waiting to be
// run over some input. Holding the interpreter open between runs is what makes
// replaying a strategy script affordable; holding the input in a field is what makes
// those runs see different data.
type preparedScript[Input any] struct {
	interpreter *interp.Interpreter
	shape       indicatorScriptShape
	// visibleInput is what the script sees. Its address is in the interpreter's
	// symbol table, so assigning to it changes the script's input without re-reading
	// a single line of the script.
	visibleInput     []Input
	parameterReader  *strategyScriptParameterReader
	executionTimeout time.Duration
}

// runOver runs the already-read script over exactly this input. Running the entry
// point through the interpreter rather than calling it directly is what makes both the
// giving up and the reporting possible: the interpreter turns every failure, including
// a deliberate one, into an error rather than letting it escape.
func (preparedScript *preparedScript[Input]) runOver(
	executionContext context.Context, input []Input,
) (map[string]vo.IndicatorValueVo, error) {
	preparedScript.visibleInput = input

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

	return preparedScript.shape.readValues(calculated)
}
