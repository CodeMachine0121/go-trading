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

func aPacedReplayRequest(
	replayStart time.Time, sources ...dto.ResolvedSignalSourceDto,
) dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:                        "BTCUSDT",
		StartTime:                     replayStart,
		EndTime:                       replayStart.Add(24 * time.Hour),
		SignalSources:                 sources,
		BuyCondition:                  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: string(vo.SignalBuy)},
		SellCondition:                 dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: string(vo.SignalSell)},
		InitialCapital:                decimal.NewFromInt(10000),
		PositionSizingMode:            string(vo.PositionSizingModeAllIn),
		TradingStrategyMarketDataKind: string(vo.MarketDataKindKCandle),
	}
}

func aSourceLookingBack(label string, interval vo.AggregationIntervalVo, lookbackCount int) dto.ResolvedSignalSourceDto {
	source := dto.ResolvedSignalSourceDto{
		Label:               label,
		AggregationInterval: string(interval),
		Script:              "//",
		MarketDataKind:      string(vo.MarketDataKindKCandle),
	}
	if lookbackCount > 0 {
		source.Parameters = []dto.StrategyScriptParameterWriteDto{
			{Name: "回看", Kind: "lookbackCount", DefaultValue: float64(lookbackCount)},
		}
	}

	return source
}

func TestTradingStrategyBacktestReadsEnoughForTheCoarsestSourceOnItsFirstPaceBar(t *testing.T) {
	replayStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	backtestDomain, buildError := domains.NewTradingStrategyBacktestDomain(aPacedReplayRequest(replayStart,
		aSourceLookingBack("A", vo.AggregationIntervalFifteenMinutes, 100),
		aSourceLookingBack("B", vo.AggregationIntervalOneDay, 80),
	), 50000, replayStart.Add(48*time.Hour))

	require.NoError(t, buildError)
	assert.Equal(t, replayStart.Add(-80*24*time.Hour), backtestDomain.KCandleQuery().StartTime())
}

func TestTradingStrategyBacktestStartsTheReadOnAWholeCoarsestBar(t *testing.T) {
	replayStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	backtestDomain, buildError := domains.NewTradingStrategyBacktestDomain(aPacedReplayRequest(replayStart,
		aSourceLookingBack("A", vo.AggregationIntervalFifteenMinutes, 10),
		aSourceLookingBack("B", vo.AggregationIntervalOneDay, 0),
	), 50000, replayStart.Add(48*time.Hour))

	require.NoError(t, buildError)
	assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), backtestDomain.KCandleQuery().StartTime())
}

func TestTradingStrategyBacktestOnOneIntervalReadsAsBefore(t *testing.T) {
	replayStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	backtestDomain, buildError := domains.NewTradingStrategyBacktestDomain(aPacedReplayRequest(replayStart,
		aSourceLookingBack("A", vo.AggregationIntervalFifteenMinutes, 100),
		aSourceLookingBack("B", vo.AggregationIntervalFifteenMinutes, 40),
	), 50000, replayStart.Add(48*time.Hour))

	require.NoError(t, buildError)
	assert.Equal(t, replayStart.Add(-100*15*time.Minute), backtestDomain.KCandleQuery().StartTime())
}

func TestTradingStrategyBacktestRefusesACoarseLookbackBeyondOneReplay(t *testing.T) {
	replayStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	_, buildError := domains.NewTradingStrategyBacktestDomain(aPacedReplayRequest(replayStart,
		aSourceLookingBack("A", vo.AggregationIntervalFifteenMinutes, 10),
		aSourceLookingBack("B", vo.AggregationIntervalOneDay, 600),
	), 50000, replayStart.Add(48*time.Hour))

	require.ErrorIs(t, buildError, domains.ErrBacktestValidation)
	assert.ErrorContains(t, buildError, "57600")
	assert.ErrorContains(t, buildError, "最多 50000 根")
	assert.ErrorContains(t, buildError, "回看根數")
	assert.ErrorContains(t, buildError, "縮短重演期間")
}
