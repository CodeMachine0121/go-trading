package script_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eatsSixteenGigabytesScript asks for far more memory than any compartment is allowed,
// all at once — the one thing the allowance cannot stop in time.
const eatsSixteenGigabytesScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	buffer := make([]byte, 16<<30)
	buffer[len(buffer)-1] = 1
	return map[string]float64{"size": float64(len(buffer))}
}
`

// requireMemoryCap skips a case that depends on the operating system enforcing the
// memory cap faithfully. That is only promised on Linux, which is what the service runs
// on. It is also skipped under the race detector: the detector keeps shadow memory that
// counts against the cap several times over, so neither what fits nor how running out
// is reported would be what the shipped binary sees. The pipeline proves these cases in
// a run of its own without the detector.
func requireMemoryCap(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("the compartment memory cap is only promised on Linux")
	}
	if raceDetectorOn {
		t.Skip("the race detector's shadow memory counts against the compartment memory cap")
	}
}

func TestCompartmentStopsAScriptThatEatsPastTheMemoryCap(t *testing.T) {
	requireMemoryCap(t)

	t.Run("a single calculation fails and names the cap", func(t *testing.T) {
		indicatorValues, err := spotIndicatorScriptProxy(10*time.Second).Execute(
			t.Context(), eatsSixteenGigabytesScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100), noStrategyScriptParameters(t))

		require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "超出記憶體上限")
		assert.Contains(t, err.Error(), "512MB")
		assert.Nil(t, indicatorValues)
	})

	t.Run("a replay fails as a whole with no partial result", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(10*time.Second).ExecuteForEachCandle(
			t.Context(), eatsSixteenGigabytesScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "超出記憶體上限")
		assert.Nil(t, perCandleIndicatorValues)
	})

	t.Run("another script running at the same moment is untouched", func(t *testing.T) {
		hungryErrors := make(chan error, 1)
		go func() {
			_, hungryError := spotIndicatorScriptProxy(10*time.Second).Execute(
				t.Context(), eatsSixteenGigabytesScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100), noStrategyScriptParameters(t))
			hungryErrors <- hungryError
		}()

		indicatorValues, err := spotIndicatorScriptProxy(10*time.Second).Execute(
			t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))
		hungryError := <-hungryErrors

		require.ErrorIs(t, hungryError, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, hungryError.Error(), "超出記憶體上限")
		require.NoError(t, err)
		assert.Equal(t, 120.0, numberOf(indicatorValues, "close"))
	})

	t.Run("the next calculation right after one ran out of memory works", func(t *testing.T) {
		_, hungryError := spotIndicatorScriptProxy(10*time.Second).Execute(
			t.Context(), eatsSixteenGigabytesScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100), noStrategyScriptParameters(t))
		require.ErrorIs(t, hungryError, domains.ErrIndicatorScriptFailed)

		indicatorValues, err := spotIndicatorScriptProxy(10*time.Second).Execute(
			t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		require.NoError(t, err)
		assert.Equal(t, 120.0, numberOf(indicatorValues, "close"))
	})
}

func TestCompartmentLetsAScriptUseMemoryWithinTheCap(t *testing.T) {
	requireMemoryCap(t)

	// Fifty megabytes, every page of it actually written, so the memory is really
	// held rather than merely promised.
	const usesFiftyMegabytesScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	buffer := make([]byte, 50<<20)
	for index := 0; index < len(buffer); index += 4096 {
		buffer[index] = 1
	}
	return map[string]float64{"done": 1}
}
`

	indicatorValues, err := spotIndicatorScriptProxy(10*time.Second).Execute(
		t.Context(), usesFiftyMegabytesScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100), noStrategyScriptParameters(t))

	require.NoError(t, err)
	assert.Equal(t, 1.0, numberOf(indicatorValues, "done"))
}

func TestCompartmentLeavesNothingRunningAfterGivingUp(t *testing.T) {
	neverEndingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	total := 0.0
	for {
		total++
	}
	return map[string]float64{"never": total}
}
`
	goroutinesBefore := runtime.NumGoroutine()

	_, timeoutError := spotIndicatorScriptProxy(300*time.Millisecond).Execute(
		t.Context(), neverEndingScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100), noStrategyScriptParameters(t))

	callerContext, leave := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer leave()
	_, callerGoneError := spotIndicatorScriptProxy(10*time.Second).Execute(
		callerContext, neverEndingScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100), noStrategyScriptParameters(t))

	require.ErrorIs(t, timeoutError, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, timeoutError.Error(), "未能算完")
	require.ErrorIs(t, callerGoneError, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, callerGoneError.Error(), "發動它的請求已經結束")
	// Every compartment has been waited for, so none of the watching this process
	// did on its behalf is still going. Counted from this goroutine, because any
	// helper that polls from one of its own would be counted along with the rest.
	settleBy := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > goroutinesBefore && time.Now().Before(settleBy) {
		time.Sleep(50 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), goroutinesBefore)
}

func TestCompartmentGivesEveryCandleOfALongReplayItsOwnAllowance(t *testing.T) {
	perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).ExecuteForEachCandle(
		t.Context(), candleCountScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(make([]float64, 1000)...), noStrategyScriptParameters(t))

	require.NoError(t, err)
	require.Len(t, perCandleIndicatorValues, 1000)
	assert.Equal(t, 1000.0, numberOf(perCandleIndicatorValues[999], "seen"))
}

func TestCompartmentReportsAScriptFailureInTheSameWords(t *testing.T) {
	t.Run("a script that panics on purpose fails while running", func(t *testing.T) {
		const panicsScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	panic("boom")
}
`

		_, err := spotIndicatorScriptProxy(2*time.Second).Execute(
			t.Context(), panicsScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100), noStrategyScriptParameters(t))

		require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "算式執行失敗")
	})

	t.Run("what a script prints does not spoil its answer", func(t *testing.T) {
		const printsScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	println("noise on the way out")
	print("more noise")
	return map[string]float64{"close": data[len(data)-1].Close}
}
`

		indicatorValues, err := spotIndicatorScriptProxy(2*time.Second).Execute(
			t.Context(), printsScript, resultTypeOf(t, "float"),
			candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		require.NoError(t, err)
		assert.Equal(t, 120.0, numberOf(indicatorValues, "close"))
	})
}

func TestCompartmentReportsAnyCompartmentThatGoesDownAsTheScriptFailing(t *testing.T) {
	testCases := []struct {
		name           string
		workerCommand  []string
		expectedReason string
	}{
		{
			name:           "a compartment that cannot even be started",
			workerCommand:  []string{"/nonexistent/indicator-script-worker"},
			expectedReason: "算式隔間無法啟動",
		},
		{
			name:           "a compartment that ends without an answer",
			workerCommand:  []string{"/bin/sh", "-c", "exit 3"},
			expectedReason: "算式隔間意外結束",
		},
		{
			// How a compartment that ran into the cap goes down, whatever system the
			// tests happen to run on: the runtime says so on its way out.
			name:           "a compartment that went down for want of memory",
			workerCommand:  []string{"/bin/sh", "-c", "echo 'fatal error: runtime: out of memory' >&2; exit 2"},
			expectedReason: "超出記憶體上限（512MB）",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			isolation := isolationWith(2 * time.Second)
			isolation.WorkerCommand = testCase.workerCommand

			indicatorValues, err := script.NewYaegiIndicatorScriptProxy(isolation).Execute(
				t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100), noStrategyScriptParameters(t))

			require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
			assert.Contains(t, err.Error(), testCase.expectedReason)
			assert.Nil(t, indicatorValues)
		})
	}
}

func TestCompartmentStopsWaitingForACompartmentThatNeverAnswers(t *testing.T) {
	// A child that never says anything at all — not even that its script ran out of
	// time — is waited for no longer than the script's allowance plus a grace period.
	isolation := isolationWith(100 * time.Millisecond)
	isolation.WorkerCommand = []string{"/bin/sh", "-c", "sleep 60"}

	startedAt := time.Now()
	_, err := script.NewYaegiIndicatorScriptProxy(isolation).Execute(
		t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100), noStrategyScriptParameters(t))
	elapsed := time.Since(startedAt)

	require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "未能算完")
	assert.Less(t, elapsed, 30*time.Second)
}
