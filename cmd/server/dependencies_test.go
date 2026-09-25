package main

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBackgroundJobsForRespectsTheSwitch(t *testing.T) {
	testCases := []struct {
		name             string
		switchValue      string
		expectedJobCount int
	}{
		{name: "switched off leaves nothing to start", switchValue: "false", expectedJobCount: 0},
		{
			name:        "switched on assembles the work the system does on its own",
			switchValue: "true", expectedJobCount: 7,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", testCase.switchValue)

			backgroundJobs := backgroundJobsFor(
				config.Load(), nil, nil, nil, nil, contractSeriesApplications{})

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}

func TestBackgroundJobsForLeavesOutAContractSeriesJobSwitchedOff(t *testing.T) {
	testCases := []struct {
		name             string
		switchedOff      string
		expectedJobCount int
	}{
		{name: "資金費率", switchedOff: "CONTRACT_FUNDING_RATE_INGESTION_INTERVAL_MINUTES", expectedJobCount: 6},
		{name: "持倉統計", switchedOff: "CONTRACT_POSITION_STATISTIC_INGESTION_INTERVAL_MINUTES", expectedJobCount: 6},
		{name: "交易規格", switchedOff: "CONTRACT_TRADING_SPECIFICATION_REFRESH_INTERVAL_HOURS", expectedJobCount: 6},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", "true")
			t.Setenv(testCase.switchedOff, "0")

			backgroundJobs := backgroundJobsFor(
				config.Load(), nil, nil, nil, nil, contractSeriesApplications{})

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}

func TestBackgroundJobsForRefreshesTheMaintenanceMarginLadderOnlyWithAnAccount(t *testing.T) {
	testCases := []struct {
		name             string
		apiKey           string
		apiSecret        string
		interval         string
		expectedJobCount int
	}{
		{name: "沒有帳戶金鑰", expectedJobCount: 7},
		{name: "只有一半的金鑰", apiKey: "key", expectedJobCount: 7},
		{name: "有帳戶金鑰", apiKey: "key", apiSecret: "secret", expectedJobCount: 8},
		{name: "有金鑰但停用", apiKey: "key", apiSecret: "secret", interval: "0", expectedJobCount: 7},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", "true")
			t.Setenv("CONTRACT_ACCOUNT_API_KEY", testCase.apiKey)
			t.Setenv("CONTRACT_ACCOUNT_API_SECRET", testCase.apiSecret)
			t.Setenv("CONTRACT_MAINTENANCE_MARGIN_TIER_REFRESH_INTERVAL_HOURS", testCase.interval)

			backgroundJobs := backgroundJobsFor(
				config.Load(), nil, nil, nil, nil, contractSeriesApplications{})

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}
