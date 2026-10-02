package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadNamesTheReplica(t *testing.T) {
	hostname, _ := os.Hostname()
	testCases := []struct {
		name         string
		replicaName  string
		expectedName string
	}{
		{name: "an explicit name is taken as given", replicaName: "go-trading-0", expectedName: "go-trading-0"},
		{name: "without one the host name is used, which is the pod name in Kubernetes", replicaName: "",
			expectedName: hostname},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("REPLICA_NAME", testCase.replicaName)

			assert.Equal(t, testCase.expectedName, config.Load().Replica.Name)
		})
	}
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
