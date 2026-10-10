package domains_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

func TestReplayAllowanceRefusalNamesWhoeverEndedTheReplay(t *testing.T) {
	compartmentsBusy := fmt.Errorf("%w: 算式隔間目前全數忙碌中，請稍後再試", domains.ErrIndicatorScriptCompartmentsBusy)
	testCases := []struct {
		name           string
		allowance      time.Duration
		callerGivesUp  bool
		executionError error
		expectedError  error
	}{
		{name: "the allowance running out", allowance: time.Millisecond,
			executionError: errors.New("stopped"), expectedError: domains.ErrBacktestTimeAllowanceSpent},
		{name: "the allowance running out while waiting for a compartment", allowance: time.Millisecond,
			executionError: compartmentsBusy, expectedError: domains.ErrBacktestTimeAllowanceSpent},
		{name: "the caller giving up first keeps the replay's own words", allowance: time.Minute, callerGivesUp: true,
			executionError: compartmentsBusy, expectedError: domains.ErrIndicatorScriptCompartmentsBusy},
		{name: "no allowance leaves the caller alone to bound it", callerGivesUp: true,
			executionError: compartmentsBusy, expectedError: domains.ErrIndicatorScriptCompartmentsBusy},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			callerContext, giveUp := context.WithCancel(context.Background())
			defer giveUp()
			allowance := domains.NewReplayAllowanceDomain(testCase.allowance)
			replayContext, stopReplaying := allowance.Bound(callerContext)
			defer stopReplaying()
			if testCase.callerGivesUp {
				giveUp()
			}
			<-replayContext.Done()

			refusal := allowance.RefusalFor(replayContext, testCase.executionError)

			assert.ErrorIs(t, refusal, testCase.expectedError)
		})
	}
}
