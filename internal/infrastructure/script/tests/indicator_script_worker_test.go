package script_test

import (
	"bytes"
	"encoding/gob"
	"io"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cases talk to the worker the way the service does — header first, then the
// request — but with what the service would never send, to pin that a compartment
// always answers rather than going down. The shapes below are this test's own; the
// wire matches them to the worker's by field name, which is exactly the contract
// between the two sides.

type workerRequestHeader struct {
	MarketKind       string
	MemoryLimitBytes int64
	ExecutionTimeout time.Duration
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

// sealedInput writes the header and then any request, the way the service sends them.
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
	// No memory cap in any of these: the worker runs inside the test process here,
	// and a cap would be set on the test process itself.
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

// serveInProcess hands one request to a worker running inside this test process and
// reads its answer. No memory cap is asked for, since it would land on this process.
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

// sealedContractInput writes a contract header and request, the way the service sends
// them for a contract script.
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
	// A cap far beyond anything this process holds, so setting it on the test process
	// — which is where an in-process worker sets it — changes nothing for the tests.
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
