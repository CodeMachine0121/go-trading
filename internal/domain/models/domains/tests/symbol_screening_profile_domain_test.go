package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aScreeningProfileWrite() dto.SymbolScreeningProfileWriteDto {
	return dto.SymbolScreeningProfileWriteDto{Name: "趨勢 V3", StrategyScriptID: 108, AggregationInterval: "4h"}
}

func aContractSignalScript() dto.RunnableStrategyScriptDto {
	return dto.RunnableStrategyScriptDto{
		Script: "package main", ResultType: "signal", MarketDataKind: "contractKCandle",
		Parameters: []dto.StrategyScriptParameterWriteDto{{Name: "讀取根數", Kind: "lookbackCount", DefaultValue: 300}},
	}
}

func TestSymbolScreeningProfileFillsTheDefaults(t *testing.T) {
	profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())

	require.NoError(t, profileError)
	stored := profile.ToEntity()
	assert.Equal(t, 730, stored.HistoryDays)
	assert.Equal(t, 3, stored.SegmentCount)
	assert.Equal(t, 6, stored.MinimumTradeCount)
	assert.True(t, decimal.NewFromInt(20_000_000).Equal(stored.MinimumAverageDailyQuoteVolume))
	assert.True(t, decimal.NewFromInt(1).Equal(stored.Leverage))
	assert.True(t, decimal.RequireFromString("0.05").Equal(stored.EntryCostPercentage))
	assert.True(t, decimal.RequireFromString("0.05").Equal(stored.SlippagePercentage))
	assert.Equal(t, "longOnly", stored.TradingMode)
	assert.Empty(t, stored.BaselineSymbols)
}

func TestSymbolScreeningProfileRefusesFiguresScreeningCannotUse(t *testing.T) {
	oneSegment := 1
	testCases := []struct {
		name            string
		change          func(writeDto *dto.SymbolScreeningProfileWriteDto)
		expectedMessage string
	}{
		{name: "fifteen minutes", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.AggregationInterval = "15m"
		}, expectedMessage: "看盤間隔只能是每 4 小時或每天"},
		{name: "a single segment", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.SegmentCount = &oneSegment
		}, expectedMessage: "區段數要介於 2 到 6"},
		{name: "no name", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.Name = "  "
		}, expectedMessage: "名稱要有 1 到 128 個字"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aScreeningProfileWrite()
			testCase.change(&writeDto)

			_, profileError := domains.NewSymbolScreeningProfileDomain(7, writeDto)

			require.ErrorIs(t, profileError, domains.ErrSymbolScreeningValidation)
			assert.Contains(t, profileError.Error(), testCase.expectedMessage)
		})
	}
}

func TestSymbolScreeningProfileAcceptsOnlyAContractSignalScriptWithoutOrderFlow(t *testing.T) {
	testCases := []struct {
		name            string
		change          func(script *dto.RunnableStrategyScriptDto)
		expectedMessage string
	}{
		{name: "for spot", change: func(script *dto.RunnableStrategyScriptDto) {
			script.MarketDataKind = "kCandle"
		}, expectedMessage: "策略腳本必須吃合約行情"},
		{name: "needs order flow", change: func(script *dto.RunnableStrategyScriptDto) {
			script.Parameters = append(script.Parameters,
				dto.StrategyScriptParameterWriteDto{Name: "價位寬度", Kind: "orderFlowPriceStep", DefaultValue: 10})
		}, expectedMessage: "篩選用行情沒有逐筆成交明細"},
		{name: "only numbers", change: func(script *dto.RunnableStrategyScriptDto) {
			script.ResultType = "float"
		}, expectedMessage: "策略腳本必須產出買賣信號"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())
			require.NoError(t, profileError)
			script := aContractSignalScript()
			testCase.change(&script)

			acceptError := profile.AcceptScript(script)

			require.ErrorIs(t, acceptError, domains.ErrSymbolScreeningValidation)
			assert.Contains(t, acceptError.Error(), testCase.expectedMessage)
		})
	}

	t.Run("a contract signal script", func(t *testing.T) {
		profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())
		require.NoError(t, profileError)

		assert.NoError(t, profile.AcceptScript(aContractSignalScript()))
	})
}

func TestSymbolScreeningProfileSnapshotCarriesTheScriptAsItIsNow(t *testing.T) {
	profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())
	require.NoError(t, profileError)

	snapshot := profile.ToSnapshot(aContractSignalScript())

	assert.Equal(t, "package main", snapshot.Script)
	assert.Equal(t, 730, snapshot.HistoryDays)
	assert.Equal(t, "趨勢 V3", snapshot.ProfileName)
}
