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

// Two gates ask whether a trading strategy's sources read one coarseness: saving it
// and replaying it. They refuse in the same words, because they are refusing the same
// fact — and somebody who meets both must not be told two different things about it.
//
// This is pinned rather than commented because the two sentences living apart is
// exactly how they would drift: one gets reworded, the other does not, and nothing
// fails.
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

	// Each gate wraps the sentence in its own kind of failure — one is a trading
	// strategy that will not save, the other names the input a replay stumbled on —
	// so the shared part is what the person actually reads.
	const sharedSentence = "這一份交易策略的信號來源目前用了 1h、5m 這幾種彙總刻度"
	assert.ErrorContains(t, saveError, sharedSentence)
	assert.ErrorContains(t, replayError, sharedSentence)
}
