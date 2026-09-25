package script

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// compartmentGracePeriod is how long past the script's allowance the service waits for a compartment that cannot time itself out.
const compartmentGracePeriod = 5 * time.Second

// outOfMemoryMark is what the Go runtime prints when the memory cap is hit; it is the only way to tell out-of-memory apart from other endings.
const outOfMemoryMark = "out of memory"

// indicatorScriptCompartment runs one request in a child process so a script's crash or memory blow-up ends only the child, which is always killed and reaped.
type indicatorScriptCompartment[Input any] struct {
	isolation IndicatorScriptIsolation
	input     indicatorScriptInput
}

func (indicatorScriptCompartment indicatorScriptCompartment[Input]) execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	input []Input,
	parameters domains.StrategyScriptParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	response, runError := indicatorScriptCompartment.run(executionContext, indicatorScriptRequest[Input]{
		Script:     script,
		ResultType: string(resultType.Value()),
		Parameters: parameters.ToDtos(),
		Input:      input,
	}, 1)
	if runError != nil {
		return nil, runError
	}

	return response.values(), nil
}

// executeForEachElement runs the whole replay in one compartment so the script is read only once.
func (indicatorScriptCompartment indicatorScriptCompartment[Input]) executeForEachElement(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	input []Input,
	parameters domains.StrategyScriptParametersDomain,
) ([]map[string]vo.IndicatorValueVo, error) {
	response, runError := indicatorScriptCompartment.run(executionContext, indicatorScriptRequest[Input]{
		Script:         script,
		ResultType:     string(resultType.Value()),
		Parameters:     parameters.ToDtos(),
		ForEachElement: true,
		Input:          input,
	}, len(input))
	if runError != nil {
		return nil, runError
	}

	return response.perElementValues(), nil
}

type compartmentOutcome struct {
	response  indicatorScriptResponse
	readError error
}

// run starts one compartment; runCount scales the outermost time limit.
func (indicatorScriptCompartment indicatorScriptCompartment[Input]) run(
	executionContext context.Context, request indicatorScriptRequest[Input], runCount int,
) (indicatorScriptResponse, error) {
	isolation := indicatorScriptCompartment.isolation

	command := exec.Command(isolation.WorkerCommand[0], isolation.WorkerCommand[1:]...)
	// An explicit empty environment, since a nil Env would inherit the service's.
	command.Env = append([]string{}, isolation.WorkerEnvironment...)
	var deathNotice bytes.Buffer
	command.Stderr = &deathNotice
	// Bounds how long to wait for output to close after the child ends, in case a leftover descendant holds the pipes.
	command.WaitDelay = time.Second

	requestPipe, requestPipeError := command.StdinPipe()
	answerPipe, answerPipeError := command.StdoutPipe()
	startError := errors.Join(requestPipeError, answerPipeError)
	if startError == nil {
		startError = command.Start()
	}
	if startError != nil {
		return indicatorScriptResponse{}, fmt.Errorf(
			"%w: 算式執行失敗：算式隔間無法啟動：%v", domains.ErrIndicatorScriptFailed, startError)
	}

	// If the child dies mid-write, the failure is read from its exit rather than from this write.
	go func() {
		defer requestPipe.Close()
		encoder := gob.NewEncoder(requestPipe)
		_ = encoder.Encode(indicatorScriptRequestHeader{
			MarketKind:       indicatorScriptCompartment.input.marketKind,
			MemoryLimitBytes: isolation.MemoryLimitBytes,
			ExecutionTimeout: isolation.ExecutionTimeout,
		})
		_ = encoder.Encode(request)
	}()

	outcomes := make(chan compartmentOutcome, 1)
	go func() {
		var response indicatorScriptResponse
		readError := gob.NewDecoder(answerPipe).Decode(&response)
		outcomes <- compartmentOutcome{response: response, readError: readError}
	}()

	outermostLimit := time.NewTimer(isolation.ExecutionTimeout*time.Duration(max(runCount, 1)) + compartmentGracePeriod)
	defer outermostLimit.Stop()

	select {
	case <-executionContext.Done():
		_ = command.Process.Kill()
		_ = command.Wait()
		return indicatorScriptResponse{}, fmt.Errorf(
			"%w: 算式已中止，因為發動它的請求已經結束", domains.ErrIndicatorScriptFailed)
	case <-outermostLimit.C:
		_ = command.Process.Kill()
		_ = command.Wait()
		return indicatorScriptResponse{}, fmt.Errorf(
			"%w: 算式在 %s 內未能算完，已中止", domains.ErrIndicatorScriptFailed, isolation.ExecutionTimeout)
	case outcome := <-outcomes:
		_ = command.Wait()
		if outcome.readError == nil {
			return outcome.response, outcome.response.failure()
		}
		if strings.Contains(deathNotice.String(), outOfMemoryMark) {
			return indicatorScriptResponse{}, fmt.Errorf(
				"%w: 算式超出記憶體上限（%dMB），已中止",
				domains.ErrIndicatorScriptFailed, isolation.MemoryLimitBytes/(1<<20))
		}

		return indicatorScriptResponse{}, fmt.Errorf(
			"%w: 算式執行失敗：算式隔間意外結束", domains.ErrIndicatorScriptFailed)
	}
}
