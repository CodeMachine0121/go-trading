package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The save and replay gates must refuse mixed coarseness in the same words; pinned so the two sentences can't drift apart.
func TestBothGatesRefuseMixedCoarsenessInTheSameWords(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources[1].AggregationInterval = string(vo.AggregationIntervalFiveMinutes)

	_, saveError := domains.NewTradingStrategyDomain(writeDto)
	require.Error(t, saveError)

	replayStart := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	_, replayError := domains.NewTradingStrategyBacktestDomain(dto.TradingStrategyBacktestRequestDto{
		Symbol:    "BTCUSDT",
		StartTime: replayStart,
		EndTime:   replayStart.Add(4 * time.Hour),
		SignalSources: []dto.ResolvedSignalSourceDto{
			{Label: "A", AggregationInterval: string(vo.AggregationIntervalOneHour), Script: "//"},
			{Label: "B", AggregationInterval: string(vo.AggregationIntervalFiveMinutes), Script: "//"},
		},
		InitialCapital:     decimal.NewFromInt(10000),
		PositionSizingMode: string(vo.PositionSizingModeAllIn),
	}, 1000, replayStart.Add(24*time.Hour))
	require.Error(t, replayError)

	// Each gate wraps this shared sentence in its own sentinel.
	const sharedSentence = "這一份交易策略的信號來源目前用了 1h、5m 這幾種彙總刻度"
	assert.ErrorContains(t, saveError, sharedSentence)
	assert.ErrorContains(t, replayError, sharedSentence)
}
