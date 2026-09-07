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
			// Keeping the stored candles current, and handing out the live places of
			// markets that limit them. They are separate jobs so that a slow round
			// cannot hold up a market that has just opened.
			name:        "switched on assembles the work the system does on its own",
			switchValue: "true", expectedJobCount: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", testCase.switchValue)

			backgroundJobs := backgroundJobsFor(config.Load(), nil, nil)

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}
