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
		{name: "switched on assembles the ingestion job", switchValue: "true", expectedJobCount: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", testCase.switchValue)
			t.Setenv("KCANDLE_INGESTION_SYMBOLS", "BTCUSDT")

			backgroundJobs := backgroundJobsFor(nil, config.Load())

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}
