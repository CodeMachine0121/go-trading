package script_test

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cases send the worker malformed input to pin that it always answers; the local wire types match the worker's by field name.

type workerRequestHeader struct {
	MarketKind         string
	MemoryLimitBytes   int64
	ExecutionTimeout   time.Duration
	ProcessorTimeLimit time.Duration
}

type workerRequestParameter struct {
	Name         string
	Kind         string
	DefaultValue float64
}

type workerRequest struct {
	Script         string
	ResultType     string
	Parameters     []workerRequestParameter
	ForEachElement bool
	Input          []vo.KCandleVo
}

type workerValue struct {
	IsList     bool
	Numbers    []float64
	HasNumbers bool
}

type workerContractRequest struct {
	Script     string
	ResultType string
	Input      []vo.ContractKCandleVo
}

type workerResponse struct {
	Values                  map[string]workerValue
	PerElementValues        []map[string]workerValue
	FailureMessage          string
	HasFailure              bool
	UndeclaredParameterName string
	HasUndeclaredParameter  bool
}

func sealedInput(t *testing.T, header workerRequestHeader, requests ...workerRequest) io.Reader {
	t.Helper()
	var input bytes.Buffer
	encoder := gob.NewEncoder(&input)
	require.NoError(t, encoder.Encode(header))
	for _, request := range requests {
		require.NoError(t, encoder.Encode(request))
	}

	return &input
}

func TestWorkerAnswersEvenARequestItCannotServe(t *testing.T) {
	// No memory cap: an in-process worker would set it on the test process itself.
	spotHeader := workerRequestHeader{MarketKind: "kCandle", ExecutionTimeout: 2 * time.Second}

	testCases := []struct {
		name           string
		input          io.Reader
		expectedReason string
	}{
		{
			name:           "nothing readable arrives",
			input:          bytes.NewBufferString("not a request"),
			expectedReason: "讀不到要算的內容",
		},
		{
			name:           "the header names a kind of market nobody knows",
			input:          sealedInput(t, workerRequestHeader{MarketKind: "futuresOnTheMoon"}),
			expectedReason: "認不得的行情種類",
		},
		{
			name:           "the header arrives but the request never does",
			input:          sealedInput(t, spotHeader),
			expectedReason: "讀不到要算的內容",
		},
		{
			name: "the request declares a kind of indicator value nobody knows",
			input: sealedInput(t, spotHeader, workerRequest{
				Script: lastClosePriceScript, ResultType: "colour",
			}),
			expectedReason: "indicator script failed",
		},
		{
			name: "the request carries a knob of a kind nobody knows",
			input: sealedInput(t, spotHeader, workerRequest{
				Script: lastClosePriceScript, ResultType: "float",
				Parameters: []workerRequestParameter{{Name: "period", Kind: "colour", DefaultValue: 1}},
			}),
			expectedReason: "indicator script failed",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer

			exitCode := script.NewIndicatorScriptWorker().Serve(testCase.input, &output)

			var response workerResponse
			require.NoError(t, gob.NewDecoder(&output).Decode(&response))
			assert.Equal(t, 0, exitCode)
			assert.True(t, response.HasFailure)
			assert.Contains(t, response.FailureMessage, testCase.expectedReason)
		})
	}
}

func TestWorkerEndsWithAFailingCodeWhenItCannotEvenAnswer(t *testing.T) {
	answerReader, answerWriter := io.Pipe()
	require.NoError(t, answerReader.Close())

	exitCode := script.NewIndicatorScriptWorker().Serve(bytes.NewBufferString("not a request"), answerWriter)

	assert.Equal(t, 1, exitCode)
}

// serveInProcess runs one request in-process without a memory cap, since it would land on this process.
func serveInProcess(t *testing.T, request workerRequest) workerResponse {
	t.Helper()
	var output bytes.Buffer

	exitCode := script.NewIndicatorScriptWorker().Serve(sealedInput(t,
		workerRequestHeader{MarketKind: "kCandle", ExecutionTimeout: 2 * time.Second}, request), &output)

	require.Equal(t, 0, exitCode)
	var response workerResponse
	require.NoError(t, gob.NewDecoder(&output).Decode(&response))

	return response
}

func TestWorkerServesTheRequestItWasStartedFor(t *testing.T) {
	t.Run("a single calculation answers with its values", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: lastClosePriceScript, ResultType: "float", Input: candlesWithClosePrices(100, 110, 120),
		})

		assert.False(t, response.HasFailure)
		assert.Equal(t, []float64{120}, response.Values["close"].Numbers)
	})

	t.Run("a replay answers with one set of values per element, in order", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: lastClosePriceScript, ResultType: "float", ForEachElement: true,
			Input: candlesWithClosePrices(100, 110, 120),
		})

		require.Len(t, response.PerElementValues, 3)
		assert.Equal(t, []float64{100}, response.PerElementValues[0]["close"].Numbers)
		assert.Equal(t, []float64{120}, response.PerElementValues[2]["close"].Numbers)
	})

	t.Run("an empty series is sent as a series that is there", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]float64 {
	return map[string][]float64{"line": {}}
}
`, ResultType: "floatList", Input: candlesWithClosePrices(100),
		})

		assert.True(t, response.Values["line"].IsList)
		assert.True(t, response.Values["line"].HasNumbers)
		assert.Empty(t, response.Values["line"].Numbers)
	})

	t.Run("a knob nobody declared is answered by name", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"period": float64(indicator.LookbackCount("period"))}
}
`, ResultType: "float", Input: candlesWithClosePrices(100),
		})

		assert.True(t, response.HasUndeclaredParameter)
		assert.Equal(t, "period", response.UndeclaredParameterName)
	})

	t.Run("a declared knob is read with the value it was sent", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"period": float64(indicator.LookbackCount("period"))}
}
`, ResultType: "float", Input: candlesWithClosePrices(100),
			Parameters: []workerRequestParameter{{Name: "period", Kind: "lookbackCount", DefaultValue: 7}},
		})

		assert.Equal(t, []float64{7}, response.Values["period"].Numbers)
	})

	t.Run("a script failing inside the compartment is answered in its own words", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: "this is not go", ResultType: "float", Input: candlesWithClosePrices(100),
		})

		assert.True(t, response.HasFailure)
		assert.Contains(t, response.FailureMessage, "算式無法解讀")
	})

	t.Run("a replay failing inside the compartment is answered with no values", func(t *testing.T) {
		response := serveInProcess(t, workerRequest{
			Script: "this is not go", ResultType: "float", ForEachElement: true,
			Input: candlesWithClosePrices(100),
		})

		assert.True(t, response.HasFailure)
		assert.Nil(t, response.PerElementValues)
	})
}

func TestWorkerServesAContractRequest(t *testing.T) {
	var output bytes.Buffer

	exitCode := script.NewIndicatorScriptWorker().Serve(sealedContractInput(t, workerContractRequest{
		Script: `
package main

import "indicator"

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	return map[string]float64{"rate": data[len(data)-1].FundingRate}
}
`,
		ResultType: "float",
		Input:      []vo.ContractKCandleVo{{FundingRate: 0.0001}, {FundingRate: 0.0003}},
	}), &output)

	var response workerResponse
	require.NoError(t, gob.NewDecoder(&output).Decode(&response))
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, []float64{0.0003}, response.Values["rate"].Numbers)
}

func sealedContractInput(t *testing.T, request workerContractRequest) io.Reader {
	t.Helper()
	var input bytes.Buffer
	encoder := gob.NewEncoder(&input)
	require.NoError(t, encoder.Encode(workerRequestHeader{
		MarketKind: "contractKCandle", ExecutionTimeout: 2 * time.Second,
	}))
	require.NoError(t, encoder.Encode(request))

	return &input
}

func TestWorkerCapsItsMemoryBeforeServing(t *testing.T) {
	// A cap far above this process's usage, so setting it in-process changes nothing.
	var output bytes.Buffer

	exitCode := script.NewIndicatorScriptWorker().Serve(sealedInput(t,
		workerRequestHeader{MarketKind: "kCandle", ExecutionTimeout: 2 * time.Second, MemoryLimitBytes: 1 << 50},
		workerRequest{Script: lastClosePriceScript, ResultType: "float", Input: candlesWithClosePrices(120)},
	), &output)

	var response workerResponse
	require.NoError(t, gob.NewDecoder(&output).Decode(&response))
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, []float64{120}, response.Values["close"].Numbers)
}

func TestWorkerIsStoppedByTheProcessorTimeLimitWithoutAnyHelpFromItsParent(t *testing.T) {
	// The allowance is an hour so only the processor time limit can end the script.
	compartment := exec.Command(os.Args[0])
	compartment.Env = []string{workerRoleVariable + "=1"}
	compartment.Stdin = sealedInput(t,
		workerRequestHeader{MarketKind: "kCandle", ExecutionTimeout: time.Hour, ProcessorTimeLimit: time.Second},
		workerRequest{Script: spinsForeverScript, ResultType: "float", Input: candlesWithClosePrices(100)},
	)
	var answer bytes.Buffer
	compartment.Stdout = &answer
	require.NoError(t, compartment.Start())

	ended := make(chan error, 1)
	go func() { ended <- compartment.Wait() }()
	select {
	case <-ended:
	case <-time.After(30 * time.Second):
		_ = compartment.Process.Kill()
		<-ended
		t.Fatal("the compartment outlived its processor time limit")
	}

	assert.False(t, compartment.ProcessState.Success())
	assert.Zero(t, answer.Len())
	assert.GreaterOrEqual(t,
		compartment.ProcessState.UserTime()+compartment.ProcessState.SystemTime(), 900*time.Millisecond)
}
