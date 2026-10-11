package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	warmupStretchStart = time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC)
	warmupNow          = time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
)

// warmupStretchBarCount is the four-hour bars from 2025-06-09 up to the bucket 2025-06-11 begins.
const warmupStretchBarCount = 12

func fourHourlyRequestLookingBack(lookbackCount int) dto.BacktestRequestDto {
	requestDto := dto.BacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "4h",
		StartTime:           warmupStretchStart,
		EndTime:             warmupStretchStart.AddDate(0, 0, 2),
		Script:              "the script",
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "allIn",
	}
	if lookbackCount > 0 {
		requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
			{Name: "回看根數", Kind: "lookbackCount", DefaultValue: float64(lookbackCount)},
		}
	}

	return requestDto
}

// fourHourlyCandlesAround stores the given number of bars before the stretch and every bar of it, all at one price.
func fourHourlyCandlesAround(barCountBefore int) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, barCountBefore+warmupStretchBarCount)
	for barIndex := -barCountBefore; barIndex < warmupStretchBarCount; barIndex++ {
		price := decimal.NewFromInt(100 + int64(barIndex+barCountBefore))
		kCandles = append(kCandles, entities.KCandle{
			Symbol:   "BTCUSDT",
			OpenTime: warmupStretchStart.Add(time.Duration(barIndex) * 4 * time.Hour),
			Open:     price, High: price, Low: price, Close: price,
		})
	}

	return kCandles
}

// signalsAt says hold everywhere except the given positions.
func signalsAt(length int, saying map[int]vo.SignalVo) []domains.SignalDomain {
	signals := make([]domains.SignalDomain, 0, length)
	for index := range length {
		signal, isSaid := saying[index]
		if !isSaid {
			signal = vo.SignalHold
		}
		signals = append(signals, domains.NewSignalDomainOf(signal))
	}

	return signals
}

func TestBacktestReadsTheDeclaredLookBackBeforeTheStretch(t *testing.T) {
	testCases := []struct {
		name                  string
		lookbackCount         int
		defaultWarmupBarCount int
		expectedReadStart     time.Time
	}{
		{
			name: "1,500 four-hour bars before 2025-06-09", lookbackCount: 1500, defaultWarmupBarCount: 200,
			expectedReadStart: time.Date(2024, 10, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "a script declaring nothing warms up by the system default", defaultWarmupBarCount: 200,
			expectedReadStart: warmupStretchStart.Add(-200 * 4 * time.Hour),
		},
		{
			name:              "no declaration and no default reads the stretch alone",
			expectedReadStart: warmupStretchStart,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := fourHourlyRequestLookingBack(testCase.lookbackCount)
			requestDto.DefaultWarmupBarCount = testCase.defaultWarmupBarCount

			backtestDomain, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, warmupNow)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedReadStart, backtestDomain.KCandleQuery().StartTime())
		})
	}
}

func TestBacktestTradesFromTheStretchStartOnceWarmedUp(t *testing.T) {
	backtestDomain, err := domains.NewBacktestDomain(
		fourHourlyRequestLookingBack(1500), backtestMaxCandleCount, warmupNow)
	require.NoError(t, err)
	inputKCandles, selectionError := backtestDomain.SelectInputCandles(fourHourlyCandlesAround(1500))
	require.NoError(t, selectionError)
	require.Len(t, inputKCandles, 1500+warmupStretchBarCount)

	result := backtestDomain.ReplayOver(inputKCandles,
		signalsAt(len(inputKCandles), map[int]vo.SignalVo{1500: vo.SignalBuy, 1501: vo.SignalSell}), nil)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, warmupStretchStart, result.ClosedTrades[0].EntryTime.Truncate(24*time.Hour))
	assert.Equal(t, warmupStretchStart, result.StartTime)
	assert.Equal(t, warmupStretchBarCount, result.UsedCandleCount)
}

func TestBacktestRunsWithWhateverHistoryThereIsBeforeTheStretch(t *testing.T) {
	backtestDomain, err := domains.NewBacktestDomain(
		fourHourlyRequestLookingBack(1500), backtestMaxCandleCount, warmupNow)
	require.NoError(t, err)

	inputKCandles, selectionError := backtestDomain.SelectInputCandles(fourHourlyCandlesAround(300))

	require.NoError(t, selectionError)
	require.Len(t, inputKCandles, 300+warmupStretchBarCount)
	result := backtestDomain.ReplayOver(inputKCandles, signalsAt(len(inputKCandles), nil), nil)
	assert.Equal(t, warmupStretchStart, result.StartTime)
	assert.Len(t, result.EquityCurve, warmupStretchBarCount)
}

func TestBacktestNeverTradesTheWarmUp(t *testing.T) {
	backtestDomain, err := domains.NewBacktestDomain(
		fourHourlyRequestLookingBack(300), backtestMaxCandleCount, warmupNow)
	require.NoError(t, err)
	inputKCandles, selectionError := backtestDomain.SelectInputCandles(fourHourlyCandlesAround(300))
	require.NoError(t, selectionError)

	result := backtestDomain.ReplayOver(inputKCandles,
		signalsAt(len(inputKCandles), map[int]vo.SignalVo{100: vo.SignalBuy, 200: vo.SignalSell, 250: vo.SignalBuy}), nil)

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.PositionOpenCount)
	assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity))
	require.Len(t, result.EquityCurve, warmupStretchBarCount)
	assert.Equal(t, warmupStretchStart, result.EquityCurve[0].OpenTime)
}

func TestBacktestNeedsTwoBarsInsideTheStretchWhateverTheWarmUp(t *testing.T) {
	requestDto := fourHourlyRequestLookingBack(300)
	backtestDomain, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, warmupNow)
	require.NoError(t, err)
	onlyWarmUpAndOneBar := fourHourlyCandlesAround(300)[:301]

	_, selectionError := backtestDomain.SelectInputCandles(onlyWarmUpAndOneBar)

	assert.ErrorIs(t, selectionError, domains.ErrBacktestValidation)
}

func TestContractBacktestNeverTradesTheWarmUpAndReportsFromTheStretch(t *testing.T) {
	requestDto := contractReplayRequest()
	requestDto.AggregationInterval = "1h"
	requestDto.StartTime = contractReplayStart.Add(3 * time.Hour)
	requestDto.EndTime = contractReplayStart.Add(8 * time.Hour)
	requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
		{Name: "回看根數", Kind: "lookbackCount", DefaultValue: 3},
	}
	contractBacktestDomain, err := domains.NewContractBacktestDomain(
		requestDto, contractReplayRules(t, contractReplaySpecification()), 100000, contractReplayStart.Add(30*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, contractReplayStart, contractBacktestDomain.KCandleQuery().StartTime())

	bars := []contractReplayBar{
		{close: 100, signal: vo.SignalBuy}, {close: 120, signal: vo.SignalSell}, {close: 90},
		{close: 100}, {close: 100}, {close: 100}, {close: 100}, {close: 100},
	}
	alignment, selectionError := contractBacktestDomain.SelectInput(contractReplayCandles("1h", bars))
	require.NoError(t, selectionError)
	signals := make([]domains.SignalDomain, 0, len(bars))
	for _, bar := range bars {
		signal := bar.signal
		if signal == "" {
			signal = vo.SignalHold
		}
		signals = append(signals, domains.NewSignalDomainOf(signal))
	}

	result := contractBacktestDomain.ReplayOver(alignment, signals, nil, nil)

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.PositionOpenCount)
	assert.Equal(t, 5, result.UsedCandleCount)
	require.NotEmpty(t, result.EquityCurve)
	assert.Equal(t, contractReplayStart.Add(3*time.Hour), result.EquityCurve[0].OpenTime)
}

func TestBacktestSplitForValidationStartsEachPartAfterTheWarmUp(t *testing.T) {
	validationStart := warmupStretchStart.Add(6 * 4 * time.Hour)
	testCases := []struct {
		name                  string
		lookbackCount         int
		defaultWarmupBarCount int
	}{
		{name: "warmed up by the declared look-back", lookbackCount: 300},
		{name: "warmed up by the system default", defaultWarmupBarCount: 300},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := fourHourlyRequestLookingBack(testCase.lookbackCount)
			requestDto.DefaultWarmupBarCount = testCase.defaultWarmupBarCount
			requestDto.ValidationStartTime = validationStart
			backtestDomain, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, warmupNow)
			require.NoError(t, err)
			inputKCandles, selectionError := backtestDomain.SelectInputCandles(fourHourlyCandlesAround(300))
			require.NoError(t, selectionError)

			result := backtestDomain.ReplayOver(inputKCandles,
				signalsAt(len(inputKCandles), map[int]vo.SignalVo{100: vo.SignalBuy, 200: vo.SignalSell}), nil)

			require.NotNil(t, result.InSample)
			require.NotNil(t, result.Validation)
			assert.Equal(t, warmupStretchStart, result.InSample.StartTime)
			assert.Equal(t, 6, result.InSample.UsedCandleCount)
			require.NotEmpty(t, result.InSample.EquityCurve)
			assert.Equal(t, warmupStretchStart, result.InSample.EquityCurve[0].OpenTime)
			assert.Equal(t, validationStart, result.Validation.StartTime)
			assert.Equal(t, warmupStretchBarCount-6, result.Validation.UsedCandleCount)
			require.NotEmpty(t, result.Validation.EquityCurve)
			assert.Equal(t, validationStart, result.Validation.EquityCurve[0].OpenTime)
			assert.Empty(t, result.InSample.ClosedTrades)
			assert.Empty(t, result.Validation.ClosedTrades)
		})
	}
}

func TestContractBacktestSplitForValidationStartsEachPartAfterTheWarmUp(t *testing.T) {
	stretchStart := contractReplayStart.Add(3 * time.Hour)
	validationStart := contractReplayStart.Add(6 * time.Hour)
	testCases := []struct {
		name                  string
		lookbackCount         int
		defaultWarmupBarCount int
	}{
		{name: "warmed up by the declared look-back", lookbackCount: 3},
		{name: "warmed up by the system default", defaultWarmupBarCount: 3},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.AggregationInterval = "1h"
			requestDto.StartTime = stretchStart
			requestDto.EndTime = contractReplayStart.Add(8 * time.Hour)
			requestDto.ValidationStartTime = validationStart
			requestDto.DefaultWarmupBarCount = testCase.defaultWarmupBarCount
			if testCase.lookbackCount > 0 {
				requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
					{Name: "回看根數", Kind: "lookbackCount", DefaultValue: float64(testCase.lookbackCount)},
				}
			}
			contractBacktestDomain, err := domains.NewContractBacktestDomain(
				requestDto, contractReplayRules(t, contractReplaySpecification()), 100000,
				contractReplayStart.Add(30*24*time.Hour))
			require.NoError(t, err)
			require.Equal(t, contractReplayStart, contractBacktestDomain.KCandleQuery().StartTime())
			bars := []contractReplayBar{
				{close: 100, signal: vo.SignalBuy}, {close: 120, signal: vo.SignalSell}, {close: 90},
				{close: 100}, {close: 100}, {close: 100}, {close: 100}, {close: 100},
			}
			alignment, selectionError := contractBacktestDomain.SelectInput(contractReplayCandles("1h", bars))
			require.NoError(t, selectionError)
			signals := make([]domains.SignalDomain, 0, len(bars))
			for _, bar := range bars {
				signal := bar.signal
				if signal == "" {
					signal = vo.SignalHold
				}
				signals = append(signals, domains.NewSignalDomainOf(signal))
			}

			result := contractBacktestDomain.ReplayOver(alignment, signals, nil, nil)

			require.NotNil(t, result.InSample)
			require.NotNil(t, result.Validation)
			assert.Equal(t, stretchStart, result.InSample.StartTime)
			assert.Equal(t, 3, result.InSample.UsedCandleCount)
			require.NotEmpty(t, result.InSample.EquityCurve)
			assert.Equal(t, stretchStart, result.InSample.EquityCurve[0].OpenTime)
			assert.Equal(t, validationStart, result.Validation.StartTime)
			assert.Equal(t, 2, result.Validation.UsedCandleCount)
			require.NotEmpty(t, result.Validation.EquityCurve)
			assert.Equal(t, validationStart, result.Validation.EquityCurve[0].OpenTime)
			assert.Empty(t, result.InSample.ClosedTrades)
			assert.Empty(t, result.Validation.ClosedTrades)
		})
	}
}

func TestTradingStrategyWarmsUpByItsSourcesDeclaredLookBackAsASingleScriptDoes(t *testing.T) {
	testCases := []struct {
		name              string
		sourceLookbacks   []float64
		expectedReadStart time.Time
	}{
		{name: "one source declaring 50 reads 50 bars back, not the default",
			sourceLookbacks: []float64{50}, expectedReadStart: backtestStart.Add(-50 * time.Hour)},
		{name: "the furthest looking source decides", sourceLookbacks: []float64{50, 0, 80},
			expectedReadStart: backtestStart.Add(-80 * time.Hour)},
		{name: "no source declaring anything warms up by the default", sourceLookbacks: []float64{0, 0},
			expectedReadStart: backtestStart.Add(-200 * time.Hour)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := tradingStrategyBacktestRequest()
			requestDto.DefaultWarmupBarCount = 200
			requestDto.SignalSources = nil
			for sourceIndex, lookbackCount := range testCase.sourceLookbacks {
				source := dto.ResolvedSignalSourceDto{
					Label: string(rune('A' + sourceIndex)), AggregationInterval: "1h", Script: "//",
				}
				if lookbackCount > 0 {
					source.Parameters = []dto.StrategyScriptParameterWriteDto{
						{Name: "回看根數", Kind: "lookbackCount", DefaultValue: lookbackCount},
					}
				}
				requestDto.SignalSources = append(requestDto.SignalSources, source)
			}

			tradingStrategyBacktestDomain, err := domains.NewTradingStrategyBacktestDomain(
				requestDto, backtestMaxCandleCount, backtestNow)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedReadStart, tradingStrategyBacktestDomain.KCandleQuery().StartTime())
		})
	}
}
