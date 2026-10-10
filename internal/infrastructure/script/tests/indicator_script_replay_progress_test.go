package script_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
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
