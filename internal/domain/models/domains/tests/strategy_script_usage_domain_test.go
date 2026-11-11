package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aStrategyScriptWriteDtoUsedAs passes every rule, so each test changes only the usage.
func aStrategyScriptWriteDtoUsedAs(usage string) dto.StrategyScriptWriteDto {
	return dto.StrategyScriptWriteDto{
		OwnerID:    7,
		Name:       "均線",
		Script:     "package main",
		ResultType: string(vo.IndicatorResultTypeFloatList),
		Usage:      usage,
	}
}

func TestNewStrategyScriptDomainSettlesTheUsageOnCreation(t *testing.T) {
	testCases := []struct {
		name          string
		declared      string
		expectedUsage vo.StrategyScriptUsageVo
	}{
		{name: "a chart script", declared: "chart", expectedUsage: vo.StrategyScriptUsageChart},
		{name: "a calculation script", declared: "calculation", expectedUsage: vo.StrategyScriptUsageCalculation},
		{name: "nothing said is a calculation script", declared: "", expectedUsage: vo.StrategyScriptUsageCalculation},
		{name: "blanks and letter case are forgiven", declared: "  Chart ", expectedUsage: vo.StrategyScriptUsageChart},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategyScript, buildError := domains.NewStrategyScriptDomain(aStrategyScriptWriteDtoUsedAs(testCase.declared))

			require.NoError(t, buildError)
			assert.Equal(t, string(testCase.expectedUsage), strategyScript.ToEntity().Usage)
		})
	}
}

func TestNewStrategyScriptDomainRefusesAUsageOutsideTheTwo(t *testing.T) {
	_, buildError := domains.NewStrategyScriptDomain(aStrategyScriptWriteDtoUsedAs("backtest"))

	require.ErrorIs(t, buildError, domains.ErrStrategyScriptValidation)
	assert.Contains(t, buildError.Error(), "策略腳本的用途只能是 chart、calculation 其中之一")
}

func TestStrategyScriptUsageDomainLetsOnlyCalculationScriptsCompute(t *testing.T) {
	testCases := []struct {
		name         string
		declared     string
		expectRefuse bool
	}{
		{name: "a calculation script computes", declared: "calculation", expectRefuse: false},
		{name: "a chart script is refused", declared: "chart", expectRefuse: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			usage, parseError := domains.NewStrategyScriptUsageDomain(testCase.declared)
			require.NoError(t, parseError)

			calculationError := usage.RequireCalculation("當信號來源")

			if !testCase.expectRefuse {
				assert.NoError(t, calculationError)
				return
			}
			require.ErrorIs(t, calculationError, domains.ErrStrategyScriptUsageNotCalculation)
			assert.Contains(t, calculationError.Error(), "這支策略腳本是圖表用，不能當信號來源")
		})
	}
}

func TestNewTradingStrategyDomainRefusesAChartScriptAsASignalSource(t *testing.T) {
	testCases := []struct {
		name         string
		usage        string
		expectRefuse bool
	}{
		{name: "a calculation signal script is a source", usage: "calculation", expectRefuse: false},
		{name: "a chart script is not", usage: "chart", expectRefuse: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aTradingStrategyWriteDto()
			writeDto.SignalSources[0].DeclaredUsage = testCase.usage

			_, buildError := domains.NewTradingStrategyDomain(writeDto)

			if !testCase.expectRefuse {
				assert.NoError(t, buildError)
				return
			}
			require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
			assert.Contains(t, buildError.Error(), "這支策略腳本是圖表用，不能當信號來源")
		})
	}
}

// The kind rule is checked first, so a calculation script that does not speak in signals keeps its old refusal.
func TestNewTradingStrategyDomainRefusesACalculationScriptWithoutSignalsAsBefore(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources[0].DeclaredUsage = "calculation"
	writeDto.SignalSources[0].DeclaredResultType = string(vo.IndicatorResultTypeFloatList)

	_, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	assert.Contains(t, buildError.Error(), "不吐訊號")
	assert.NotContains(t, buildError.Error(), "圖表用")
}

func TestSymbolScreeningProfileRefusesAChartScript(t *testing.T) {
	profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())
	require.NoError(t, profileError)
	script := aContractSignalScript()
	script.Usage = "chart"

	acceptError := profile.AcceptScript(script)

	require.ErrorIs(t, acceptError, domains.ErrSymbolScreeningValidation)
	assert.Contains(t, acceptError.Error(), "這支策略腳本是圖表用，不能用於篩選")
}

func TestStrategyScriptMarketplaceCopyTakesTheUsageItHasWhenAdopted(t *testing.T) {
	testCases := []struct {
		name  string
		usage string
	}{
		{name: "a chart script stays a chart script", usage: "chart"},
		{name: "a calculation script stays a calculation script", usage: "calculation"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			copied := domains.NewStrategyScriptMarketplaceCopyDomain(
				entities.StrategyScript{Name: "動能", Usage: testCase.usage}, 2, time.Now()).
				NamedAvoiding(map[string]bool{}).ToEntity()

			assert.Equal(t, testCase.usage, copied.Usage)
		})
	}
}

func TestPublishedStrategyScriptShowsTheScriptsCurrentUsage(t *testing.T) {
	published := entities.PublishedStrategyScript{
		StrategyScript: entities.StrategyScript{ID: 3, Usage: "chart"},
	}

	assert.Equal(t, "chart", published.ToDto().Usage)
}

// A stored usage no one can read is refused too, rather than read as either usage.
func TestAnUnreadableStoredUsageIsRefusedWhereverItIsUsed(t *testing.T) {
	t.Run("as a signal source", func(t *testing.T) {
		writeDto := aTradingStrategyWriteDto()
		writeDto.SignalSources[0].DeclaredUsage = "sideways"

		_, buildError := domains.NewTradingStrategyDomain(writeDto)

		require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
		assert.Contains(t, buildError.Error(), "用途不對")
	})

	t.Run("for screening", func(t *testing.T) {
		profile, profileError := domains.NewSymbolScreeningProfileDomain(7, aScreeningProfileWrite())
		require.NoError(t, profileError)
		script := aContractSignalScript()
		script.Usage = "sideways"

		require.ErrorIs(t, profile.AcceptScript(script), domains.ErrSymbolScreeningValidation)
	})
}
