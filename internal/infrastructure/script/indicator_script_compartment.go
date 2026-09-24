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

// compartmentGracePeriod is how long past the script's own allowance the service waits
// before it stops waiting for a compartment. The compartment gives up on a slow script
// by itself and says so; this is only for one that cannot even do that.
const compartmentGracePeriod = 5 * time.Second

// outOfMemoryMark is what the Go runtime writes on its way down when the memory cap
// refuses it more. It is the one sign, among all the ways a compartment can end, that
// tells running out of memory apart from everything else.
const outOfMemoryMark = "out of memory"

// indicatorScriptCompartment is the service's side of a script compartment: it starts a
// child process for one run — a single calculation or a whole replay — hands it the
// request, and watches it until there is an answer or a reason there will not be one.
//
// Whatever happens in the child stays there. A script that eats past the memory cap
// ends the child, not the service. A caller that goes away, or a child that outlives
// every allowance, gets the child ended — and waited for, so nothing is left running
// and no finished process is left unreaped. Every one of those endings comes back as
// the same kinds of failure the service always told scripts apart by.
type indicatorScriptCompartment[Input any] struct {
	isolation IndicatorScriptIsolation
	input     indicatorScriptInput
}

// execute runs the script once over the whole input.
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

// executeForEachElement replays the script once per element, the whole replay in one
// compartment: the script is read once there and fed a growing stretch, exactly as
// it always was.
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

// compartmentOutcome is what reading a compartment's answer came to.
type compartmentOutcome struct {
	response  indicatorScriptResponse
	readError error
}

// run starts one compartment, sends it the request, and waits for its answer. runCount
// is how many runs of the script the request asks for, which is what the outermost
// time limit is measured in.
func (indicatorScriptCompartment indicatorScriptCompartment[Input]) run(
	executionContext context.Context, request indicatorScriptRequest[Input], runCount int,
) (indicatorScriptResponse, error) {
	isolation := indicatorScriptCompartment.isolation

	command := exec.Command(isolation.WorkerCommand[0], isolation.WorkerCommand[1:]...)
	// An empty environment rather than none: leaving it unset would hand the child
	// everything the service itself was started with.
	command.Env = append([]string{}, isolation.WorkerEnvironment...)
	var deathNotice bytes.Buffer
	command.Stderr = &deathNotice
	// Once the child has been told to end, the service waits this long and no longer
	// for its output to close. Anything the child left behind still holding those
	// streams would otherwise keep the wait — and whoever is waiting on this run —
	// hanging for as long as it lived.
	command.WaitDelay = time.Second

	requestPipe, requestPipeError := command.StdinPipe()
	answerPipe, answerPipeError := command.StdoutPipe()
	startError := errors.Join(requestPipeError, answerPipeError)
	if startError == nil {
		startError = command.Start()
	}
	// A compartment that never came up ran nothing of the script, but to its author
	// it is still a run that did not happen, so it is told the same way.
	if startError != nil {
		return indicatorScriptResponse{}, fmt.Errorf(
			"%w: 算式執行失敗：算式隔間無法啟動：%v", domains.ErrIndicatorScriptFailed, startError)
	}

	// A child that dies part way through leaves this write with nowhere to go; the
	// failure is then read from how the child ended, not from here.
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
