package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func archivedStatistic() vo.ContractPositionStatisticArchiveVo {
	return vo.ContractPositionStatisticArchiveVo{
		Symbol:                          "ETHUSDT",
		StatisticTime:                   time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC),
		OpenInterest:                    figure("1794474.325"),
		OpenInterestValue:               figure("3528901866.675849"),
		AccountLongShortRatio:           figure("3"),
		TopTraderPositionLongShortRatio: figure("1"),
	}
}

func TestContractPositionStatisticArchiveDomainWorksTheSharesOutOfTheRatio(t *testing.T) {
	testCases := []struct {
		name          string
		ratio         string
		expectedLong  string
		expectedShort string
	}{
		{name: "比值 3 是四份裡多方佔三份", ratio: "3", expectedLong: "0.75", expectedShort: "0.25"},
		{name: "比值 1 是各佔一半", ratio: "1", expectedLong: "0.5", expectedShort: "0.5"},
		{name: "比值 0 是一面倒做空", ratio: "0", expectedLong: "0", expectedShort: "1"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, side := range []string{"多空人數比", "大戶多空持倉比"} {
				archived := archivedStatistic()
				if side == "多空人數比" {
					archived.AccountLongShortRatio = figure(testCase.ratio)
				} else {
					archived.TopTraderPositionLongShortRatio = figure(testCase.ratio)
				}

				archiveDomain, buildError := domains.NewContractPositionStatisticArchiveDomain(archived)

				require.NoError(t, buildError)
				converted := archiveDomain.ToContractPositionStatisticVo()
				longShare, shortShare, longShortRatio := converted.AccountLongShare,
					converted.AccountShortShare, converted.AccountLongShortRatio
				if side == "大戶多空持倉比" {
					longShare, shortShare, longShortRatio = converted.TopTraderPositionLongShare,
						converted.TopTraderPositionShortShare, converted.TopTraderPositionLongShortRatio
				}
				assert.True(t, decimal.RequireFromString(testCase.expectedLong).Equal(longShare.Decimal),
					"%s 多方佔比 %s", side, longShare.Decimal)
				assert.True(t, decimal.RequireFromString(testCase.expectedShort).Equal(shortShare.Decimal),
					"%s 空方佔比 %s", side, shortShare.Decimal)
				assert.True(t, decimal.RequireFromString(testCase.ratio).Equal(longShortRatio.Decimal),
					"%s 比值 %s", side, longShortRatio.Decimal)
			}
		})
	}
}

func TestContractPositionStatisticArchiveDomainKeepsWhatItDoesNotWorkOut(t *testing.T) {
	archiveDomain, buildError := domains.NewContractPositionStatisticArchiveDomain(archivedStatistic())

	require.NoError(t, buildError)
	converted := archiveDomain.ToContractPositionStatisticVo()
	assert.Equal(t, "ETHUSDT", converted.Symbol)
	assert.Equal(t, time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC), converted.StatisticTime)
	assert.True(t, decimal.RequireFromString("1794474.325").Equal(converted.OpenInterest))
	assert.True(t, decimal.RequireFromString("3528901866.675849").Equal(converted.OpenInterestValue))
}

func TestContractPositionStatisticArchiveDomainRefusesAReadingItCannotWorkWith(t *testing.T) {
	testCases := []struct {
		name            string
		adjust          func(archived *vo.ContractPositionStatisticArchiveVo)
		expectedMessage string
	}{
		{name: "全體帳戶比值為負", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.AccountLongShortRatio = figure("-1")
		}, expectedMessage: "多空人數比的比值不得為負"},
		{name: "大戶比值為負", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.TopTraderPositionLongShortRatio = figure("-0.5")
		}, expectedMessage: "大戶多空持倉比的比值不得為負"},
		{name: "缺持倉量", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.OpenInterest = decimal.NullDecimal{}
		}, expectedMessage: "缺持倉量"},
		{name: "缺持倉價值", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.OpenInterestValue = decimal.NullDecimal{}
		}, expectedMessage: "缺持倉價值"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			archived := archivedStatistic()
			testCase.adjust(&archived)

			_, buildError := domains.NewContractPositionStatisticArchiveDomain(archived)

			require.ErrorIs(t, buildError, domains.ErrContractPositionStatisticValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedMessage)
		})
	}
}

func TestContractPositionStatisticArchiveDomainLeavesAMissingRatioForTheLiveRulesToName(t *testing.T) {
	archived := archivedStatistic()
	archived.TopTraderPositionLongShortRatio = decimal.NullDecimal{}

	archiveDomain, buildError := domains.NewContractPositionStatisticArchiveDomain(archived)
	require.NoError(t, buildError)
	converted := archiveDomain.ToContractPositionStatisticVo()

	assert.False(t, converted.TopTraderPositionLongShare.Valid)
	assert.False(t, converted.TopTraderPositionShortShare.Valid)
	assert.False(t, converted.TopTraderPositionLongShortRatio.Valid)
	assert.True(t, decimal.RequireFromString("0.75").Equal(converted.AccountLongShare.Decimal))

	_, judgeError := domains.NewContractPositionStatisticDomain(
		converted, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC))
	require.ErrorIs(t, judgeError, domains.ErrContractPositionStatisticValidation)
	assert.Contains(t, judgeError.Error(), "缺大戶多空持倉比")
}
