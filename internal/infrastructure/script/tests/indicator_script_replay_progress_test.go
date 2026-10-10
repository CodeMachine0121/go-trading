package script_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReplayReportsEveryBarAsItIsComputed(t *testing.T) {
	progress := mocks.NewMockIReplayProgressSink(gomock.NewController(t))
	reportsMutex := sync.Mutex{}
	completedCounts := []int{}
	progress.EXPECT().ReplayAdvanced(gomock.Any(), 50).
		Do(func(completedBarCount int, _ int) {
			reportsMutex.Lock()
			defer reportsMutex.Unlock()
			completedCounts = append(completedCounts, completedBarCount)
		}).Times(50)

	perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).ExecuteForEachCandle(
		t.Context(), candleCountScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(make([]float64, 50)...), noStrategyScriptParameters(t), progress)

	require.NoError(t, err)
	require.Len(t, perCandleIndicatorValues, 50)
	assert.Equal(t, []float64{50}, perCandleIndicatorValues[49]["seen"].Numbers)
	reportsMutex.Lock()
	defer reportsMutex.Unlock()
	for index, completedCount := range completedCounts {
		assert.Equal(t, index+1, completedCount)
	}
}

func TestReplayAnsweredForFewerBarsThanItWasGivenFails(t *testing.T) {
	isolation := isolationWith(2 * time.Second)
	isolation.WorkerEnvironment = []string{shortAnswerRoleVariable + "=1"}

	testCases := []struct {
		name   string
		replay func() (int, error)
	}{
		{name: "a spot replay", replay: func() (int, error) {
			perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(isolation).ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t), nil)
			return len(perCandleIndicatorValues), err
		}},
		{name: "a contract replay", replay: func() (int, error) {
			perCandleIndicatorValues, err := script.NewYaegiContractIndicatorScriptProxy(isolation).ExecuteForEachCandle(
				t.Context(), averageCloseOfContractBarsScript, resultTypeOf(t, "float"),
				contractBarsWithClosePrices(100, 110, 120), noStrategyScriptParameters(t), nil)
			return len(perCandleIndicatorValues), err
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			answeredCount, err := testCase.replay()

			require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
			assert.Contains(t, err.Error(), "交回 1 根的結果，應有 3 根")
			assert.Zero(t, answeredCount)
		})
	}
}
