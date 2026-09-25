package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var archiveJudgedAt = time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

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
	ratios := []struct {
		name          string
		ratio         string
		expectedLong  string
		expectedShort string
	}{
		{name: "比值 3 是四份裡多方佔三份", ratio: "3", expectedLong: "0.75", expectedShort: "0.25"},
		{name: "比值 1 是各佔一半", ratio: "1", expectedLong: "0.5", expectedShort: "0.5"},
		{name: "比值 0 是一面倒做空", ratio: "0", expectedLong: "0", expectedShort: "1"},
	}
	sides := []struct {
		name     string
		setRatio func(archived *vo.ContractPositionStatisticArchiveVo, ratio decimal.NullDecimal)
		storedOf func(stored entities.ContractPositionStatistic) (longShare, shortShare, ratio decimal.Decimal)
	}{
		{name: "多空人數比",
			setRatio: func(archived *vo.ContractPositionStatisticArchiveVo, ratio decimal.NullDecimal) {
				archived.AccountLongShortRatio = ratio
			},
			storedOf: func(stored entities.ContractPositionStatistic) (decimal.Decimal, decimal.Decimal, decimal.Decimal) {
				return stored.AccountLongShare, stored.AccountShortShare, stored.AccountLongShortRatio
			}},
		{name: "大戶多空持倉比",
			setRatio: func(archived *vo.ContractPositionStatisticArchiveVo, ratio decimal.NullDecimal) {
				archived.TopTraderPositionLongShortRatio = ratio
			},
			storedOf: func(stored entities.ContractPositionStatistic) (decimal.Decimal, decimal.Decimal, decimal.Decimal) {
				return stored.TopTraderPositionLongShare, stored.TopTraderPositionShortShare,
					stored.TopTraderPositionLongShortRatio
			}},
	}

	for _, ratioCase := range ratios {
		for _, side := range sides {
			t.Run(ratioCase.name+"／"+side.name, func(t *testing.T) {
				archived := archivedStatistic()
				side.setRatio(&archived, figure(ratioCase.ratio))

				archiveDomain, buildError := domains.NewContractPositionStatisticArchiveDomain(archived, archiveJudgedAt)

				require.NoError(t, buildError)
				longShare, shortShare, longShortRatio := side.storedOf(archiveDomain.ToEntity())
				assert.True(t, decimal.RequireFromString(ratioCase.expectedLong).Equal(longShare), "多方佔比 %s", longShare)
				assert.True(t, decimal.RequireFromString(ratioCase.expectedShort).Equal(shortShare), "空方佔比 %s", shortShare)
				assert.True(t, decimal.RequireFromString(ratioCase.ratio).Equal(longShortRatio), "比值 %s", longShortRatio)
			})
		}
	}
}

func TestContractPositionStatisticArchiveDomainKeepsWhatItDoesNotWorkOut(t *testing.T) {
	archiveDomain, buildError := domains.NewContractPositionStatisticArchiveDomain(archivedStatistic(), archiveJudgedAt)

	require.NoError(t, buildError)
	stored := archiveDomain.ToEntity()
	assert.Equal(t, "ETHUSDT", stored.Symbol)
	assert.Equal(t, time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC), stored.StatisticTime)
	assert.True(t, decimal.RequireFromString("1794474.325").Equal(stored.OpenInterest))
	assert.True(t, decimal.RequireFromString("3528901866.675849").Equal(stored.OpenInterestValue))
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
		// The rest are the live rules, applied to the worked-out reading unchanged.
		{name: "缺大戶比值", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.TopTraderPositionLongShortRatio = decimal.NullDecimal{}
		}, expectedMessage: "缺大戶多空持倉比"},
		{name: "缺全體帳戶比值", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.AccountLongShortRatio = decimal.NullDecimal{}
		}, expectedMessage: "缺多空人數比"},
		{name: "持倉量為負", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.OpenInterest = figure("-1")
		}, expectedMessage: "持倉量不得為負"},
		{name: "統計時間不在五分鐘刻度", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.StatisticTime = time.Date(2026, 3, 1, 9, 3, 0, 0, time.UTC)
		}, expectedMessage: "五分鐘刻度"},
		{name: "統計時間指向未來", adjust: func(archived *vo.ContractPositionStatisticArchiveVo) {
			archived.StatisticTime = archiveJudgedAt.Add(5 * time.Minute)
		}, expectedMessage: "未來"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			archived := archivedStatistic()
			testCase.adjust(&archived)

			_, buildError := domains.NewContractPositionStatisticArchiveDomain(archived, archiveJudgedAt)

			require.ErrorIs(t, buildError, domains.ErrContractPositionStatisticValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedMessage)
		})
	}
}
