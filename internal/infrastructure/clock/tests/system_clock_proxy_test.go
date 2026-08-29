package clock_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/stretchr/testify/assert"
)

func TestSystemClockProxyNow(t *testing.T) {
	systemClockProxy := clock.NewSystemClockProxy()

	reportedTime := systemClockProxy.Now()

	assert.Equal(t, time.UTC, reportedTime.Location())
	assert.WithinDuration(t, time.Now().UTC(), reportedTime, time.Second)
}
