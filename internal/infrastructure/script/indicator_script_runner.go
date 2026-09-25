package script

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// errScriptAllowanceSpent is the deadline cause, distinguishing a timed-out script from a caller that went away.
var errScriptAllowanceSpent = errors.New("indicator script allowance spent")

const scriptEntryPoint = "main.Calculate"

// scriptCall runs the entry point through the interpreter so a script that never finishes can be stopped.
const scriptCall = "main.Calculate(indicator.Data)"

const scriptDataPackage = "indicator/indicator"

// allowedPackages is the only set of imports a script may use: no files, network, clock or randomness.
var allowedPackages = interp.Exports{
	"math/math": stdlib.Symbols["math/math"],
	"sort/sort": stdlib.Symbols["sort/sort"],
}

// indicatorScriptRunner runs a script over one input element type (spot K candles or contract bars) inside a compartment, sharing the sandbox, allowance and failure reporting across markets.
type indicatorScriptRunner[Input any] struct {
	executionTimeout time.Duration
	input            indicatorScriptInput
}

// execute runs the script once; any failure is reported as a script failure with no partial result.
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

	// A copy of exact length, so the script can neither mutate the caller's input nor read past it.
	isolatedInput := make([]Input, len(input))
	copy(isolatedInput, input)

	return preparedScript.runOver(executionContext, isolatedInput)
}

// executeForEachElement reads the script once and runs it per element over a growing prefix, each with its own allowance; the first failure aborts with no partial result.
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

	// Each run sees a copy capped to its length, so the script cannot re-slice to read or rewrite future candles.
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

// prepare builds the interpreter, evaluates the source and checks the entry-point signature, so broken scripts fail before any data is read.
func (runner indicatorScriptRunner[Input]) prepare(
	script string,
	resultType domains.IndicatorResultTypeDomain,
	parameters domains.StrategyScriptParametersDomain,
) (*preparedScript[Input], error) {
	// The reader records the first undeclared parameter so the report does not depend on the interpreter's panic wording.
	preparedScript := &preparedScript[Input]{
		// Script output is discarded because the compartment's stdout is the response channel.
		interpreter: interp.New(interp.Options{Stdout: io.Discard, Stderr: io.Discard}),
		shape: indicatorScriptShape{
			resultType:     resultType,
			inputSliceType: reflect.TypeOf([]Input(nil)),
		},
		parameterReader:  &strategyScriptParameterReader{parameters: parameters},
		executionTimeout: runner.executionTimeout,
	}

	dataSymbols := map[string]reflect.Value{
		// The field's address (not a copy) is exported so its contents can be swapped between runs.
		"Data":          reflect.ValueOf(&preparedScript.visibleInput).Elem(),
		"LookbackCount": reflect.ValueOf(preparedScript.parameterReader.lookbackCount),
		"Number":        reflect.ValueOf(preparedScript.parameterReader.number),
		"Boolean":       reflect.ValueOf(preparedScript.parameterReader.boolean),
		// Signal has no constructor, so a signal-kind script can only return one of these three values.
		"Signal": reflect.ValueOf((*vo.SignalVo)(nil)),
		"Buy":    reflect.ValueOf(vo.SignalBuy),
		"Sell":   reflect.ValueOf(vo.SignalSell),
		"Hold":   reflect.ValueOf(vo.SignalHold),
	}
	for typeName, typeSymbol := range runner.input.types {
		dataSymbols[typeName] = typeSymbol
	}

	scriptSymbols := interp.Exports{scriptDataPackage: dataSymbols}
	for packagePath, packageSymbols := range allowedPackages {
		scriptSymbols[packagePath] = packageSymbols
	}

	// The symbol table is built from constants and cannot fail; a malformed one would surface as an unreadable script anyway.
	_ = preparedScript.interpreter.Use(scriptSymbols)

	// Goroutines and channels are rejected because they can outlive the allowance or crash the server with an uncatchable panic; the check fails closed, retrying scripts without a package clause as package main.
	parsedScript, parseError := parser.ParseFile(token.NewFileSet(), "", script, 0)
	if parseError != nil {
		parsedAsMain, parseAsMainError := parser.ParseFile(
			token.NewFileSet(), "", "package main\n"+script, 0)
		if parseAsMainError != nil {
			return nil, fmt.Errorf(
				"%w: 算式無法解讀：%v", domains.ErrIndicatorScriptFailed, parseError)
		}
		parsedScript = parsedAsMain
	}

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
			domains.ErrIndicatorScriptFailed, resultType.Value(), runner.input.typeName,
			resultType.ScriptResultShape())
	}

	return preparedScript, nil
}

// preparedScript keeps the interpreter open between runs and swaps the input via a field.
type preparedScript[Input any] struct {
	interpreter      *interp.Interpreter
	shape            indicatorScriptShape
	visibleInput     []Input
	parameterReader  *strategyScriptParameterReader
	executionTimeout time.Duration
}

// runOver runs the prepared script through the interpreter, which turns every failure, including panics, into an error.
func (preparedScript *preparedScript[Input]) runOver(
	executionContext context.Context, input []Input,
) (map[string]vo.IndicatorValueVo, error) {
	preparedScript.visibleInput = input

	// Derived from the caller's context so a departed caller stops the script too.
	allowanceContext, stopWaiting := context.WithTimeoutCause(
		executionContext, preparedScript.executionTimeout, errScriptAllowanceSpent)
	defer stopWaiting()

	calculated, callError := preparedScript.interpreter.EvalWithContext(allowanceContext, scriptCall)

	// Checked first so an undeclared parameter is reported as such rather than as whatever error it caused.
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

// answer runs the compartment's single request and always returns a response, since only a response can cross back.
func (runner indicatorScriptRunner[Input]) answer(decoder *gob.Decoder) indicatorScriptResponse {
	var request indicatorScriptRequest[Input]
	if decodeError := decoder.Decode(&request); decodeError != nil {
		return newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：算式隔間讀不到要算的內容：%v", domains.ErrIndicatorScriptFailed, decodeError))
	}

	resultType, resultTypeError := domains.NewIndicatorResultTypeDomain(request.ResultType)
	if resultTypeError != nil {
		return newFailedIndicatorScriptResponse(resultTypeError)
	}

	parameterWrites := make([]dto.StrategyScriptParameterWriteDto, 0, len(request.Parameters))
	for _, parameter := range request.Parameters {
		parameterWrites = append(parameterWrites, parameter.ToWriteDto())
	}
	parameters, parametersError := domains.NewStrategyScriptParametersDomain(parameterWrites)
	if parametersError != nil {
		return newFailedIndicatorScriptResponse(parametersError)
	}

	// The caller cannot cancel from inside the compartment; the service kills the whole process instead.
	if request.ForEachElement {
		perElementIndicatorValues, replayError := runner.executeForEachElement(
			context.Background(), request.Script, resultType, request.Input, parameters)
		if replayError != nil {
			return newFailedIndicatorScriptResponse(replayError)
		}

		perElementValues := make([]indicatorValuesWire, 0, len(perElementIndicatorValues))
		for _, indicatorValues := range perElementIndicatorValues {
			perElementValues = append(perElementValues, newIndicatorValuesWire(indicatorValues))
		}

		return indicatorScriptResponse{PerElementValues: perElementValues}
	}

	indicatorValues, executionError := runner.execute(
		context.Background(), request.Script, resultType, request.Input, parameters)
	if executionError != nil {
		return newFailedIndicatorScriptResponse(executionError)
	}

	return indicatorScriptResponse{Values: newIndicatorValuesWire(indicatorValues)}
}
