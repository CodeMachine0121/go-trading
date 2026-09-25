package script_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const spinsForeverScript = `
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

const panicsOnPurposeScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	panic("boom")
}
`

func isolationWithSlots(executionTimeout time.Duration, slots *script.IndicatorScriptCompartmentSlots) script.IndicatorScriptIsolation {
	isolation := isolationWith(executionTimeout)
	isolation.CompartmentSlots = slots

	return isolation
}

func calculateLastClosePrice(
	executionContext context.Context, t *testing.T, isolation script.IndicatorScriptIsolation,
) (float64, error) {
	indicatorValues, err := script.NewYaegiIndicatorScriptProxy(isolation).Execute(
		executionContext, lastClosePriceScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))
	if err != nil {
		return 0, err
	}

	return numberOf(indicatorValues, "close"), nil
}

// occupyEverySlot fills all slots with scripts that never end; each returned function ends one of them and returns once its slot is back.
func occupyEverySlot(t *testing.T, slots *script.IndicatorScriptCompartmentSlots, capacity int) []func() {
	t.Helper()
	leaves := make([]func(), 0, capacity)
	for range capacity {
		holderContext, leave := context.WithCancel(t.Context())
		holderEnded := make(chan struct{})
		go func() {
			defer close(holderEnded)
			_, _ = script.NewYaegiIndicatorScriptProxy(isolationWithSlots(time.Minute, slots)).Execute(
				holderContext, spinsForeverScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100), noStrategyScriptParameters(t))
		}()
		leaves = append(leaves, func() {
			leave()
			<-holderEnded
		})
	}
	t.Cleanup(func() {
		for _, leave := range leaves {
			leave()
		}
	})

	// A probe that finds every slot taken proves the holders have them all.
	occupiedBy := time.Now().Add(10 * time.Second)
	for time.Now().Before(occupiedBy) {
		probeContext, stopProbing := context.WithTimeout(t.Context(), 20*time.Millisecond)
		_, probeError := calculateLastClosePrice(probeContext, t, isolationWithSlots(time.Minute, slots))
		stopProbing()
		if errors.Is(probeError, domains.ErrIndicatorScriptCompartmentsBusy) {
			return leaves
		}
	}

	t.Fatal("the holders never took every slot")
	return nil
}

func TestCompartmentSlotsLetACalculationStartWhileASlotIsFree(t *testing.T) {
	testCases := []struct {
		name          string
		occupiedCount int
	}{
		{name: "nothing else running", occupiedCount: 0},
		{name: "one of two slots taken", occupiedCount: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			slots := script.NewIndicatorScriptCompartmentSlots(2)
			if testCase.occupiedCount > 0 {
				// Filling both and freeing one leaves exactly one taken.
				occupyEverySlot(t, slots, 2)[0]()
			}

			// The holder never ends, so finishing within this deadline means nothing waited for it.
			callerContext, giveUp := context.WithTimeout(t.Context(), 5*time.Second)
			defer giveUp()
			closePrice, err := calculateLastClosePrice(callerContext, t, isolationWithSlots(time.Minute, slots))

			require.NoError(t, err)
			assert.Equal(t, 120.0, closePrice)
		})
	}
}

func TestCompartmentSlotsMakeACalculationWaitForAFreedSlot(t *testing.T) {
	slots := script.NewIndicatorScriptCompartmentSlots(1)
	leave := occupyEverySlot(t, slots, 1)[0]
	time.AfterFunc(300*time.Millisecond, leave)

	closePrice, err := calculateLastClosePrice(t.Context(), t, isolationWithSlots(time.Minute, slots))

	require.NoError(t, err)
	assert.Equal(t, 120.0, closePrice)
}

func TestCompartmentSlotsReportBusyWhenTheCallerStopsWaiting(t *testing.T) {
	slots := script.NewIndicatorScriptCompartmentSlots(1)
	occupyEverySlot(t, slots, 1)

	callerContext, giveUp := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer giveUp()
	startedAt := time.Now()
	_, err := calculateLastClosePrice(callerContext, t, isolationWithSlots(time.Minute, slots))

	require.ErrorIs(t, err, domains.ErrIndicatorScriptCompartmentsBusy)
	assert.NotErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "忙碌中")
	assert.Less(t, time.Since(startedAt), 5*time.Second)
}

func TestCompartmentSlotsAreReturnedHoweverACalculationEnds(t *testing.T) {
	testCases := []struct {
		name          string
		endPrevious   func(t *testing.T, slots *script.IndicatorScriptCompartmentSlots) error
		expectedError error
	}{
		{
			name: "the script failed on purpose",
			endPrevious: func(t *testing.T, slots *script.IndicatorScriptCompartmentSlots) error {
				_, err := script.NewYaegiIndicatorScriptProxy(isolationWithSlots(time.Minute, slots)).Execute(
					t.Context(), panicsOnPurposeScript, resultTypeOf(t, "float"),
					candlesWithClosePrices(100), noStrategyScriptParameters(t))
				return err
			},
			expectedError: domains.ErrIndicatorScriptFailed,
		},
		{
			name: "the script ran out of time",
			endPrevious: func(t *testing.T, slots *script.IndicatorScriptCompartmentSlots) error {
				_, err := script.NewYaegiIndicatorScriptProxy(isolationWithSlots(300*time.Millisecond, slots)).Execute(
					t.Context(), spinsForeverScript, resultTypeOf(t, "float"),
					candlesWithClosePrices(100), noStrategyScriptParameters(t))
				return err
			},
			expectedError: domains.ErrIndicatorScriptFailed,
		},
		{
			name: "the caller left mid-run",
			endPrevious: func(t *testing.T, slots *script.IndicatorScriptCompartmentSlots) error {
				callerContext, leave := context.WithTimeout(t.Context(), 300*time.Millisecond)
				defer leave()
				_, err := script.NewYaegiIndicatorScriptProxy(isolationWithSlots(time.Minute, slots)).Execute(
					callerContext, spinsForeverScript, resultTypeOf(t, "float"),
					candlesWithClosePrices(100), noStrategyScriptParameters(t))
				return err
			},
			expectedError: domains.ErrIndicatorScriptFailed,
		},
		{
			name: "the caller left while queueing",
			endPrevious: func(t *testing.T, slots *script.IndicatorScriptCompartmentSlots) error {
				leaveHolder := occupyEverySlot(t, slots, 1)[0]
				defer leaveHolder()
				callerContext, leave := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer leave()
				_, err := calculateLastClosePrice(callerContext, t, isolationWithSlots(time.Minute, slots))
				return err
			},
			expectedError: domains.ErrIndicatorScriptCompartmentsBusy,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			slots := script.NewIndicatorScriptCompartmentSlots(1)
			require.ErrorIs(t, testCase.endPrevious(t, slots), testCase.expectedError)

			callerContext, giveUp := context.WithTimeout(t.Context(), 5*time.Second)
			defer giveUp()
			closePrice, err := calculateLastClosePrice(callerContext, t, isolationWithSlots(time.Minute, slots))

			require.NoError(t, err)
			assert.Equal(t, 120.0, closePrice)
		})
	}
}

func TestCompartmentSlotsServeStrategyBotRoundsFirst(t *testing.T) {
	testCases := []struct {
		name                         string
		laterServesStrategyBotRounds bool
		expectedFirstFinisher        string
	}{
		{name: "a bot round overtakes a queued on-demand calculation", laterServesStrategyBotRounds: true, expectedFirstFinisher: "later"},
		{name: "on-demand calculations go first come first served", laterServesStrategyBotRounds: false, expectedFirstFinisher: "earlier"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			slots := script.NewIndicatorScriptCompartmentSlots(1)
			leave := occupyEverySlot(t, slots, 1)[0]

			finishers := make(chan string, 2)
			queue := func(name string, servesStrategyBotRounds bool) {
				isolation := isolationWithSlots(time.Minute, slots)
				isolation.ServesStrategyBotRounds = servesStrategyBotRounds
				go func() {
					_, err := calculateLastClosePrice(t.Context(), t, isolation)
					if err == nil {
						finishers <- name
					}
				}()
				// Long enough for the waiter to join the queue before the next one arrives.
				time.Sleep(200 * time.Millisecond)
			}
			queue("earlier", false)
			queue("later", testCase.laterServesStrategyBotRounds)

			leave()
			firstFinisher := <-finishers
			secondFinisher := <-finishers

			assert.Equal(t, testCase.expectedFirstFinisher, firstFinisher)
			assert.NotEqual(t, firstFinisher, secondFinisher)
		})
	}
}
