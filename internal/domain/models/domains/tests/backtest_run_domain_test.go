package domains_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var backtestRunMoment = time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)

func aBacktestRunIn(status string) domains.BacktestRunDomain {
	return domains.NewBacktestRunDomain(entities.BacktestRun{ID: 12, Kind: "spotScript", Status: status})
}

func TestBacktestRunProgressIsTheShareOfBarsDone(t *testing.T) {
	testCases := []struct {
		name              string
		status            string
		completedBarCount int
		totalBarCount     int
		expectedPercent   int
	}{
		{name: "day 120 of 480", status: "running", completedBarCount: 120, totalBarCount: 480, expectedPercent: 25},
		{name: "nothing known yet", status: "queued", expectedPercent: 0},
		{name: "one bar short of the end is not yet all", status: "running", completedBarCount: 479, totalBarCount: 480,
			expectedPercent: 99},
		{name: "a succeeded run is whole", status: "succeeded", completedBarCount: 3, totalBarCount: 480,
			expectedPercent: 100},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			backtestRun := aBacktestRunIn(testCase.status).Advanced(testCase.completedBarCount, testCase.totalBarCount)

			assert.Equal(t, testCase.expectedPercent, backtestRun.ProgressPercent())
			assert.Equal(t, testCase.expectedPercent, backtestRun.ToDto().ProgressPercent)
		})
	}
}

func TestBacktestRunReadsAnUnknownStatusAsFailed(t *testing.T) {
	backtestRun := aBacktestRunIn("exploded")

	assert.Equal(t, "failed", backtestRun.ToDto().Status)
	assert.False(t, backtestRun.IsActive())
}

func TestBacktestRunCanBeCancelledOnlyWhileItHasWorkAhead(t *testing.T) {
	testCases := []struct {
		status          string
		expectedRefusal error
	}{
		{status: "queued"},
		{status: "running"},
		{status: "succeeded", expectedRefusal: domains.ErrBacktestRunAlreadyEnded},
		{status: "failed", expectedRefusal: domains.ErrBacktestRunAlreadyEnded},
		{status: "cancelled", expectedRefusal: domains.ErrBacktestRunAlreadyEnded},
	}

	for _, testCase := range testCases {
		t.Run(testCase.status, func(t *testing.T) {
			cancelled, err := aBacktestRunIn(testCase.status).Cancelled(backtestRunMoment)

			if testCase.expectedRefusal != nil {
				require.ErrorIs(t, err, testCase.expectedRefusal)
				assert.Contains(t, err.Error(), "已經結束")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "cancelled", cancelled.ToDto().Status)
			require.NotNil(t, cancelled.ToDto().FinishedAt)
			assert.Equal(t, backtestRunMoment, *cancelled.ToDto().FinishedAt)
		})
	}
}

func TestBacktestRunLifeFromClaimToEnding(t *testing.T) {
	claimed := aBacktestRunIn("queued").Claimed("replica-a", backtestRunMoment)

	t.Run("a claimed run is running under its replica", func(t *testing.T) {
		assert.Equal(t, "running", claimed.ToDto().Status)
		assert.Equal(t, "replica-a", claimed.ToEntity().ReplicaName)
		require.NotNil(t, claimed.ToDto().StartedAt)
		assert.Equal(t, backtestRunMoment, *claimed.ToDto().StartedAt)
	})

	t.Run("a success keeps the headline figures and reads complete", func(t *testing.T) {
		succeeded := claimed.Advanced(10, 480).Succeeded(dto.BacktestRunSummaryDto{
			TotalReturnRate: 0.255, MaximumDrawdown: 0.101, TradeCount: 4,
		}, backtestRunMoment.Add(time.Hour))

		runDto := succeeded.ToDto()
		assert.Equal(t, "succeeded", runDto.Status)
		assert.Equal(t, 100, runDto.ProgressPercent)
		require.NotNil(t, runDto.TotalReturnRate)
		assert.InDelta(t, 0.255, *runDto.TotalReturnRate, 1e-9)
		assert.InDelta(t, 0.101, *runDto.MaximumDrawdown, 1e-9)
		assert.Equal(t, 4, *runDto.TradeCount)
		assert.Equal(t, "", runDto.FailureReason)
	})

	t.Run("a failure keeps the reason in the person's words and no figures", func(t *testing.T) {
		failed := claimed.FailedFor(errors.New("算式執行失敗：除以零"), false, backtestRunMoment.Add(time.Hour))

		runDto := failed.ToDto()
		assert.Equal(t, "failed", runDto.Status)
		assert.Equal(t, "算式執行失敗：除以零", runDto.FailureReason)
		assert.Nil(t, runDto.TotalReturnRate)
		assert.Nil(t, runDto.TradeCount)
		require.NotNil(t, runDto.FinishedAt)
		assert.Equal(t, backtestRunMoment.Add(time.Hour), *runDto.FinishedAt)
		assert.False(t, failed.IsActive())
	})
}

func TestBacktestRunFailureNamesWhyInThePersonsWords(t *testing.T) {
	testCases := []struct {
		name           string
		replayError    error
		interrupted    bool
		expectedReason string
	}{
		{name: "the script failing keeps its own words", replayError: errors.New("算式執行失敗：除以零"),
			expectedReason: "算式執行失敗：除以零"},
		{name: "running past the allowance", replayError: domains.BacktestTimeAllowanceSpent(30 * time.Minute),
			expectedReason: "超過時間上限"},
		{name: "the replay breaking down", replayError: domains.ErrBacktestRunBrokeDown,
			expectedReason: "回測服務壞掉了，這筆沒有跑完"},
		{name: "a graceful stop reads as a restart whatever the replay said", interrupted: true,
			replayError: errors.New("算式已中止，因為發動它的請求已經結束"), expectedReason: "被重新啟動打斷"},
		{name: "a graceful stop beats the allowance", interrupted: true,
			replayError: domains.BacktestTimeAllowanceSpent(30 * time.Minute), expectedReason: "被重新啟動打斷"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failed := aBacktestRunIn("running").FailedFor(testCase.replayError, testCase.interrupted, backtestRunMoment)

			assert.Equal(t, "failed", failed.ToDto().Status)
			assert.Equal(t, testCase.expectedReason, failed.ToDto().FailureReason)
		})
	}
}

func TestSubmittedBacktestRunDescribesWhatItReplays(t *testing.T) {
	strategyScriptID, tradingStrategyID := uint(88), uint(7)
	testCases := []struct {
		name                string
		submission          dto.BacktestRunSubmissionDto
		expectedKind        string
		expectedInterval    string
		expectedLeverage    *decimal.Decimal
		expectedTradingMode *string
	}{
		{
			name: "a spot script",
			submission: dto.BacktestRunSubmissionDto{Kind: "spotScript", StrategyScriptID: &strategyScriptID,
				Request: dto.BacktestRunRequestDto{SpotScript: &dto.BacktestRequestDto{
					Symbol: "BTCUSDT", AggregationInterval: "4h",
				}}},
			expectedKind: "spotScript", expectedInterval: "4h",
		},
		{
			name: "a contract script with leverage left out reads as one times",
			submission: dto.BacktestRunSubmissionDto{Kind: "contractScript",
				Request: dto.BacktestRunRequestDto{ContractScript: &dto.ContractBacktestRequestDto{
					Symbol: "BTCUSDT", AggregationInterval: "1h",
				}}},
			expectedKind: "contractScript", expectedInterval: "1h",
			expectedLeverage: decimalPointer("1"), expectedTradingMode: stringPointer("longShort"),
		},
		{
			name: "a spot trading strategy takes its interval from its sources",
			submission: dto.BacktestRunSubmissionDto{Kind: "spotTradingStrategy", TradingStrategyID: &tradingStrategyID,
				Request: dto.BacktestRunRequestDto{SpotTradingStrategy: &dto.TradingStrategyBacktestRequestDto{
					Symbol: "BTCUSDT", SignalSources: []dto.ResolvedSignalSourceDto{{AggregationInterval: "1d"}},
				}}},
			expectedKind: "spotTradingStrategy", expectedInterval: "1d",
		},
		{
			name: "a contract trading strategy carries the strategy's own trading mode",
			submission: dto.BacktestRunSubmissionDto{Kind: "contractTradingStrategy", TradingStrategyID: &tradingStrategyID,
				Request: dto.BacktestRunRequestDto{ContractTradingStrategy: &dto.ContractTradingStrategyBacktestRequestDto{
					Symbol: "BTCUSDT", Leverage: decimal.NewFromInt(5), TradingStrategyTradingMode: "shortOnly",
					SignalSources: []dto.ResolvedSignalSourceDto{{AggregationInterval: "15m"}},
				}}},
			expectedKind: "contractTradingStrategy", expectedInterval: "15m",
			expectedLeverage: decimalPointer("5"), expectedTradingMode: stringPointer("shortOnly"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			submitted, err := domains.NewSubmittedBacktestRunDomain(3, testCase.submission, backtestRunMoment)

			require.NoError(t, err)
			runDto := submitted.ToDto()
			assert.Equal(t, testCase.expectedKind, runDto.Kind)
			assert.Equal(t, "queued", runDto.Status)
			assert.Equal(t, testCase.expectedInterval, runDto.Interval)
			assert.Equal(t, backtestRunMoment, runDto.SubmittedAt)
			assert.Equal(t, uint(3), submitted.ToEntity().OwnerID)
			if testCase.expectedLeverage == nil {
				assert.Nil(t, runDto.Leverage)
				assert.Nil(t, runDto.TradingMode)
				return
			}
			require.NotNil(t, runDto.Leverage)
			assert.True(t, testCase.expectedLeverage.Equal(*runDto.Leverage))
			assert.Equal(t, *testCase.expectedTradingMode, *runDto.TradingMode)
		})
	}
}

func TestSubmittedBacktestRunRefusesASubmissionItCannotDescribe(t *testing.T) {
	testCases := []struct {
		name       string
		submission dto.BacktestRunSubmissionDto
	}{
		{name: "a kind nobody knows", submission: dto.BacktestRunSubmissionDto{Kind: "optionScript"}},
		{name: "a spot script without its request", submission: dto.BacktestRunSubmissionDto{Kind: "spotScript"}},
		{name: "a contract script without its request", submission: dto.BacktestRunSubmissionDto{Kind: "contractScript"}},
		{name: "a spot strategy without sources", submission: dto.BacktestRunSubmissionDto{Kind: "spotTradingStrategy",
			Request: dto.BacktestRunRequestDto{SpotTradingStrategy: &dto.TradingStrategyBacktestRequestDto{}}}},
		{name: "a contract strategy without its request", submission: dto.BacktestRunSubmissionDto{
			Kind: "contractTradingStrategy"}},
		{name: "a contract script with a trading mode nobody knows", submission: dto.BacktestRunSubmissionDto{
			Kind: "contractScript", Request: dto.BacktestRunRequestDto{ContractScript: &dto.ContractBacktestRequestDto{
				TradingMode: "sideways"}}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewSubmittedBacktestRunDomain(3, testCase.submission, backtestRunMoment)

			assert.ErrorIs(t, err, domains.ErrBacktestValidation)
		})
	}
}

func TestBacktestRunAdmissionCapsWhatOneOwnerHasQueuedOrRunning(t *testing.T) {
	testCases := []struct {
		name             string
		ownerActiveLimit int
		ownerActiveCount int
		expectedRefused  bool
		expectedLimit    int
	}{
		{name: "two of three", ownerActiveLimit: 3, ownerActiveCount: 2},
		{name: "three of three", ownerActiveLimit: 3, ownerActiveCount: 3, expectedRefused: true, expectedLimit: 3},
		{name: "an unusable limit reads as the default of three", ownerActiveLimit: 0, ownerActiveCount: 3,
			expectedRefused: true, expectedLimit: 3},
		{name: "a negative limit too", ownerActiveLimit: -1, ownerActiveCount: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := domains.NewBacktestRunAdmissionDomain(testCase.ownerActiveLimit).Admit(testCase.ownerActiveCount)

			if !testCase.expectedRefused {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, domains.ErrBacktestRunOwnerLimitReached)
			limit, isLimit := domains.BacktestRunOwnerLimit(err)
			assert.True(t, isLimit)
			assert.Equal(t, testCase.expectedLimit, limit)
			assert.Equal(t, "你同時最多 3 筆回測在排隊或進行中，等其中一筆結束再送", err.Error())
		})
	}
}

func decimalPointer(value string) *decimal.Decimal {
	parsed := decimal.RequireFromString(value)

	return &parsed
}

func stringPointer(value string) *string {
	return &value
}
