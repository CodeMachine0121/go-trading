package entities_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKCandleContractHistorySyncRunKeepsTheCandlesAndTheStatisticsApart(t *testing.T) {
	runDto := entities.KCandleContractHistorySyncRun{
		StoredCount: 1000, SkippedCount: 4, FetchFailureReason: "candles refused",
		PositionStatisticTotalDays: 181, PositionStatisticCompletedDays: 181,
		PositionStatisticStoredCount: 500, PositionStatisticSkippedCount: 2,
		PositionStatisticFetchFailureReason: "archive refused",
	}.ToDto()

	assert.Equal(t, 1000, runDto.StoredCount)
	assert.Equal(t, 4, runDto.SkippedCount)
	assert.Equal(t, "candles refused", runDto.FetchFailureReason)
	assert.Equal(t, dto.ContractPositionStatisticSyncProgressDto{
		TotalDays: 181, CompletedDays: 181, StoredCount: 500, SkippedCount: 2,
		FetchFailureReason: "archive refused",
	}, runDto.PositionStatistic)
}

func TestKCandleContractHistorySyncRunFromBeforeTheStatisticsHasNoneOfThem(t *testing.T) {
	runDto := entities.KCandleContractHistorySyncRun{StoredCount: 30, TotalChunks: 3}.ToDto()

	assert.Equal(t, dto.ContractPositionStatisticSyncProgressDto{}, runDto.PositionStatistic)
	assert.Equal(t, 30, runDto.StoredCount)
}

func TestKCandleContractHistorySyncRunLeavesTheCandleFiguresWhereCallersAlreadyReadThem(t *testing.T) {
	encoded, encodeError := json.Marshal(entities.KCandleContractHistorySyncRun{
		ID: 7, StoredCount: 1000, PositionStatisticTotalDays: 181, PositionStatisticStoredCount: 500,
	}.ToDto())
	require.NoError(t, encodeError)

	decoded := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.JSONEq(t, `7`, string(decoded["id"]))
	assert.JSONEq(t, `1000`, string(decoded["storedCount"]))
	assert.JSONEq(t, `{"totalDays":181,"completedDays":0,"storedCount":500,"skippedCount":0,"fetchFailureReason":""}`,
		string(decoded["positionStatistic"]))
}
