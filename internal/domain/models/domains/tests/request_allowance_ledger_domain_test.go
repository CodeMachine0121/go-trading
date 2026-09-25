package domains_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	generalBudget    = vo.RequestBudgetVo{RequestsPerMinute: 600, Burst: 120}
	credentialBudget = vo.RequestBudgetVo{RequestsPerMinute: 10, Burst: 10}
	firstRequestAt   = time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
)

type admission struct {
	requesterKey string
	after        time.Duration
}

func TestRequestAllowanceLedgerAdmitsUntilTheAllowanceIsSpent(t *testing.T) {
	testCases := []struct {
		name               string
		budget             vo.RequestBudgetVo
		earlier            []admission
		then               admission
		expectedRetryAfter time.Duration
		expectedMessage    string
	}{
		{
			name:    "the saved-up allowance covers a burst",
			budget:  generalBudget,
			earlier: repeated("address:A", 0, 119),
			then:    admission{requesterKey: "address:A"},
		},
		{
			name:               "the request past the burst waits a tenth of a second",
			budget:             generalBudget,
			earlier:            repeated("address:A", 0, 120),
			then:               admission{requesterKey: "address:A"},
			expectedRetryAfter: 100 * time.Millisecond,
			expectedMessage:    "請求太頻繁，請 1 秒後再試",
		},
		{
			name:    "a refused request spends nothing, so the next share arrives on time",
			budget:  generalBudget,
			earlier: repeated("address:A", 0, 121),
			then:    admission{requesterKey: "address:A", after: 100 * time.Millisecond},
		},
		{
			name:    "one requester running out does not touch another",
			budget:  generalBudget,
			earlier: repeated("address:A", 0, 121),
			then:    admission{requesterKey: "address:D"},
		},
		{
			name:               "the credential budget waits six seconds past its ten",
			budget:             credentialBudget,
			earlier:            repeated("address:A", 0, 10),
			then:               admission{requesterKey: "address:A"},
			expectedRetryAfter: 6 * time.Second,
			expectedMessage:    "請求太頻繁，請 6 秒後再試",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ledger := domains.NewRequestAllowanceLedgerDomain(testCase.budget)
			for _, earlier := range testCase.earlier {
				_ = ledger.Admit(earlier.requesterKey, firstRequestAt.Add(earlier.after))
			}

			admitError := ledger.Admit(testCase.then.requesterKey, firstRequestAt.Add(testCase.then.after))

			if testCase.expectedRetryAfter == 0 {
				assert.NoError(t, admitError)
				return
			}
			var rateExceeded domains.RequestRateExceededError
			require.True(t, errors.As(admitError, &rateExceeded))
			assert.ErrorIs(t, admitError, domains.ErrRequestRateExceeded)
			assert.Equal(t, testCase.expectedRetryAfter, rateExceeded.RetryAfter)
			assert.Equal(t, testCase.expectedMessage, admitError.Error())
		})
	}
}

func TestRequestAllowanceLedgerForgetsOnlyRequestersWhoseAllowanceHasRefilled(t *testing.T) {
	testCases := []struct {
		name                  string
		budget                vo.RequestBudgetVo
		earlier               []admission
		trigger               admission
		expectedTrackedCount  int
		thenFromA             int
		expectedAdmittedFromA int
	}{
		{
			name:                 "ten thousand one-off sources are forgotten once refilled",
			budget:               generalBudget,
			earlier:              distinct(10000, 0),
			trigger:              admission{requesterKey: "address:Z", after: time.Minute},
			expectedTrackedCount: 1,
		},
		{
			name:   "a source still refilling is remembered and keeps what it spent",
			budget: credentialBudget,
			earlier: append(
				[]admission{{requesterKey: "address:X"}},
				repeated("address:A", 30*time.Second, 10)...),
			trigger:               admission{requesterKey: "address:Y", after: time.Minute},
			expectedTrackedCount:  2,
			thenFromA:             6,
			expectedAdmittedFromA: 5,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ledger := domains.NewRequestAllowanceLedgerDomain(testCase.budget)
			for _, earlier := range testCase.earlier {
				require.NoError(t, ledger.Admit(earlier.requesterKey, firstRequestAt.Add(earlier.after)))
			}

			require.NoError(t, ledger.Admit(testCase.trigger.requesterKey, firstRequestAt.Add(testCase.trigger.after)))
			assert.Equal(t, testCase.expectedTrackedCount, ledger.TrackedRequesterCount())

			admittedFromA := 0
			for range testCase.thenFromA {
				if ledger.Admit("address:A", firstRequestAt.Add(testCase.trigger.after)) == nil {
					admittedFromA++
				}
			}
			assert.Equal(t, testCase.expectedAdmittedFromA, admittedFromA)
		})
	}
}

func repeated(requesterKey string, after time.Duration, count int) []admission {
	admissions := make([]admission, 0, count)
	for range count {
		admissions = append(admissions, admission{requesterKey: requesterKey, after: after})
	}

	return admissions
}

func distinct(count int, after time.Duration) []admission {
	admissions := make([]admission, 0, count)
	for index := range count {
		admissions = append(admissions, admission{requesterKey: "address:" + strconv.Itoa(index), after: after})
	}

	return admissions
}
