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
