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

var statisticCurrentTime = time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

func reportedStatistic() vo.ContractPositionStatisticVo {
	return vo.ContractPositionStatisticVo{
		Symbol:                          "BTCUSDT",
		StatisticTime:                   time.Date(2026, 9, 23, 9, 5, 0, 0, time.UTC),
		OpenInterest:                    decimal.RequireFromString("106479.865"),
		OpenInterestValue:               decimal.RequireFromString("9262820059.48"),
		AccountLongShare:                figure("0.47"),
		AccountShortShare:               figure("0.53"),
		AccountLongShortRatio:           figure("0.89"),
		TopTraderPositionLongShare:      figure("0.6688"),
		TopTraderPositionShortShare:     figure("0.3312"),
		TopTraderPositionLongShortRatio: figure("2.0189"),
	}
}

func TestNewContractPositionStatisticDomainKeepsALawfulStatistic(t *testing.T) {
	testCases := []struct {
		name          string
		adjust        func(statistic *vo.ContractPositionStatisticVo)
		expectedLong  string
		expectedShort string
	}{
		{name: "一般的一筆", adjust: func(*vo.ContractPositionStatisticVo) {},
			expectedLong: "0.47", expectedShort: "0.53"},
		{name: "一面倒", adjust: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.AccountLongShare = figure("1")
			statistic.AccountShortShare = figure("0")
		}, expectedLong: "1", expectedShort: "0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			statistic := reportedStatistic()
			testCase.adjust(&statistic)

			statisticDomain, buildError := domains.NewContractPositionStatisticDomain(statistic, statisticCurrentTime)

			require.NoError(t, buildError)
			stored := statisticDomain.ToEntity()
			assert.Equal(t, "BTCUSDT", stored.Symbol)
			assert.Equal(t, time.Date(2026, 9, 23, 9, 5, 0, 0, time.UTC), stored.StatisticTime)
			assert.True(t, decimal.RequireFromString("106479.865").Equal(stored.OpenInterest))
			assert.True(t, decimal.RequireFromString("9262820059.48").Equal(stored.OpenInterestValue))
			assert.True(t, decimal.RequireFromString(testCase.expectedLong).Equal(stored.AccountLongShare))
			assert.True(t, decimal.RequireFromString(testCase.expectedShort).Equal(stored.AccountShortShare))
			assert.True(t, decimal.RequireFromString("0.89").Equal(stored.AccountLongShortRatio))
			assert.True(t, decimal.RequireFromString("0.6688").Equal(stored.TopTraderPositionLongShare))
			assert.True(t, decimal.RequireFromString("0.3312").Equal(stored.TopTraderPositionShortShare))
			assert.True(t, decimal.RequireFromString("2.0189").Equal(stored.TopTraderPositionLongShortRatio))
		})
	}
}

func TestNewContractPositionStatisticDomainRefusesAnUnlawfulStatistic(t *testing.T) {
	testCases := []struct {
		name            string
		breakStatistic  func(statistic *vo.ContractPositionStatisticVo)
		expectedMessage string
	}{
		{name: "佔比超過一", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.AccountLongShare = figure("1.2")
		}, expectedMessage: "佔比必須介於零與一之間"},
		{name: "空方佔比為負", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.TopTraderPositionShortShare = figure("-0.1")
		}, expectedMessage: "大戶多空持倉比的佔比必須介於零與一之間"},
		{name: "比值為負", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.TopTraderPositionLongShortRatio = figure("-1")
		}, expectedMessage: "大戶多空持倉比的比值不得為負"},
		{name: "持倉量為負", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.OpenInterest = decimal.RequireFromString("-1")
		}, expectedMessage: "持倉量不得為負"},
		{name: "持倉價值為負", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.OpenInterestValue = decimal.RequireFromString("-1")
		}, expectedMessage: "持倉價值不得為負"},
		{name: "不在五分鐘刻度", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.StatisticTime = time.Date(2026, 9, 23, 9, 3, 0, 0, time.UTC)
		}, expectedMessage: "統計時間必須落在五分鐘刻度"},
		{name: "指向未來", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.StatisticTime = statisticCurrentTime.Add(5 * time.Minute)
		}, expectedMessage: "統計時間不得指向未來"},
		{name: "缺大戶多空持倉比", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.TopTraderPositionLongShortRatio = decimal.NullDecimal{}
		}, expectedMessage: "缺大戶多空持倉比"},
		{name: "缺多空人數比", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.AccountShortShare = decimal.NullDecimal{}
		}, expectedMessage: "缺多空人數比"},
		{name: "沒有代號", breakStatistic: func(statistic *vo.ContractPositionStatisticVo) {
			statistic.Symbol = ""
		}, expectedMessage: "必須指定交易標的"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			statistic := reportedStatistic()
			testCase.breakStatistic(&statistic)

			_, buildError := domains.NewContractPositionStatisticDomain(statistic, statisticCurrentTime)

			assert.ErrorIs(t, buildError, domains.ErrContractPositionStatisticValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestContractPositionStatisticWindowReachesAnHourBehindTheLastHeldStatistic(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	testCases := []struct {
		name          string
		currentTime   time.Time
		latestHeld    time.Time
		hasLatest     bool
		expectedStart time.Time
		expectedEnd   time.Time
		expectedEmpty bool
	}{
		{name: "接著上一次,並重問它前面那一小時", currentTime: now, latestHeld: time.Date(2026, 9, 23, 9, 5, 0, 0, time.UTC), hasLatest: true,
			expectedStart: time.Date(2026, 9, 23, 8, 10, 0, 0, time.UTC), expectedEnd: now},
		{name: "重問那一小時也不越過三十天", currentTime: now, latestHeld: thirtyDaysAgo.Add(15 * time.Minute), hasLatest: true,
			expectedStart: thirtyDaysAgo.Add(5 * time.Minute), expectedEnd: now},
		{name: "從沒存過就從三十天內第一格開始", currentTime: now, hasLatest: false,
			expectedStart: thirtyDaysAgo.Add(5 * time.Minute), expectedEnd: now},
		{name: "上一次早於三十天也一樣", currentTime: now, latestHeld: thirtyDaysAgo.Add(-10 * 24 * time.Hour), hasLatest: true,
			expectedStart: thirtyDaysAgo.Add(5 * time.Minute), expectedEnd: now},
		{name: "現在不在刻度上時終點退回上一格", currentTime: now.Add(2 * time.Minute),
			latestHeld: time.Date(2026, 9, 23, 9, 5, 0, 0, time.UTC), hasLatest: true,
			expectedStart: time.Date(2026, 9, 23, 8, 10, 0, 0, time.UTC), expectedEnd: now},
		{name: "已經存到最新那一格仍重問最近一小時", currentTime: now.Add(2 * time.Minute), latestHeld: now, hasLatest: true,
			expectedStart: now.Add(-55 * time.Minute), expectedEnd: now},
		{name: "還沒有任何一格能問", currentTime: now, latestHeld: now.Add(2 * time.Hour), hasLatest: true,
			expectedStart: now.Add(65 * time.Minute), expectedEnd: now, expectedEmpty: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := domains.NewContractPositionStatisticWindowDomain(
				testCase.currentTime, testCase.latestHeld, testCase.hasLatest)

			assert.Equal(t, testCase.expectedStart, window.StartTime())
			assert.Equal(t, testCase.expectedEnd, window.EndTime())
			assert.Equal(t, testCase.expectedEmpty, window.IsEmpty())
		})
	}
}

func TestContractPositionStatisticWindowNeverReachesTheVenuesEdgeExactly(t *testing.T) {
	// Thirty days before a moment off the grid: the first grid moment after it.
	now := time.Date(2026, 9, 23, 10, 2, 0, 0, time.UTC)

	window := domains.NewContractPositionStatisticWindowDomain(now, time.Time{}, false)

	assert.Equal(t, time.Date(2026, 8, 24, 10, 5, 0, 0, time.UTC), window.StartTime())
	assert.Equal(t, time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC), window.EndTime())
}
