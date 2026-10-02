package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadNamesTheReplica(t *testing.T) {
	t.Setenv("REPLICA_NAME", "go-trading-0")

	assert.Equal(t, "go-trading-0", config.Load().Replica.Name)
}

func TestLoadGivesEveryProcessOnAHostItsOwnName(t *testing.T) {
	hostname, _ := os.Hostname()
	t.Setenv("REPLICA_NAME", "")

	firstName := config.Load().Replica.Name
	secondName := config.Load().Replica.Name

	assert.True(t, strings.HasPrefix(firstName, hostname+"-"), firstName)
	assert.NotEqual(t, firstName, secondName, "two processes on one host must not share a name")
}

func TestLoadAppliesTheJobLeadershipDefaults(t *testing.T) {
	jobLeadership := config.Load().JobLeadership

	assert.Equal(t, 30*time.Second, jobLeadership.LeaseDuration)
	assert.Equal(t, 10*time.Second, jobLeadership.RenewInterval)
	assert.Equal(t, 5*time.Second, jobLeadership.SafetyMargin)
}

func TestLoadAppliesThePendingMessageDefaults(t *testing.T) {
	pendingMessage := config.Load().PendingMessage

	assert.Equal(t, 2*time.Second, pendingMessage.DispatchInterval)
	assert.Equal(t, 2*time.Minute, pendingMessage.SendTimeout)
	assert.Equal(t, 8, pendingMessage.MaxConcurrentDeliveries)
}

func TestLoadCorrectsTimingSettingsThatContradictEachOther(t *testing.T) {
	testCases := []struct {
		name                string
		variables           map[string]string
		expectedRenew       time.Duration
		expectedMargin      time.Duration
		expectedSendTimeout time.Duration
	}{
		{
			name:          "a renewal slower than the duty is trusted for is brought inside it",
			variables:     map[string]string{"JOB_LEADERSHIP_RENEW_INTERVAL_SECONDS": "40"},
			expectedRenew: 12500 * time.Millisecond, expectedMargin: 5 * time.Second, expectedSendTimeout: 2 * time.Minute,
		},
		{
			name:          "a margin as long as the lease is cut back",
			variables:     map[string]string{"JOB_LEADERSHIP_SAFETY_MARGIN_SECONDS": "30"},
			expectedRenew: 10 * time.Second, expectedMargin: 5 * time.Second, expectedSendTimeout: 2 * time.Minute,
		},
		{
			name:          "a send timeout shorter than two Telegram requests is lengthened",
			variables:     map[string]string{"PENDING_MESSAGE_SEND_TIMEOUT_SECONDS": "15"},
			expectedRenew: 10 * time.Second, expectedMargin: 5 * time.Second, expectedSendTimeout: 30 * time.Second,
		},
		{
			name: "settings that agree are taken as given", variables: map[string]string{},
			expectedRenew: 10 * time.Second, expectedMargin: 5 * time.Second, expectedSendTimeout: 2 * time.Minute,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for name, value := range testCase.variables {
				t.Setenv(name, value)
			}

			applicationConfig := config.Load()

			assert.Equal(t, testCase.expectedRenew, applicationConfig.JobLeadership.RenewInterval)
			assert.Equal(t, testCase.expectedMargin, applicationConfig.JobLeadership.SafetyMargin)
			assert.Equal(t, testCase.expectedSendTimeout, applicationConfig.PendingMessage.SendTimeout)
		})
	}
}
