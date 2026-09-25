package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var admissionMoment = time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)

// Each person's token names them; the expired one fails verification like a forged one would.
var (
	xiaomingRequester = dto.RequesterDto{AccessToken: "xiaoming-token", ClientAddress: "10.42.0.20"}
	xiaohuaRequester  = dto.RequesterDto{AccessToken: "xiaohua-token", ClientAddress: "10.42.0.20"}
	expiredRequester  = dto.RequesterDto{AccessToken: "expired-token", ClientAddress: "203.0.113.5"}
	anonymousAtA      = dto.RequesterDto{ClientAddress: "203.0.113.5"}
	xiaomingAtA       = dto.RequesterDto{AccessToken: "xiaoming-token", ClientAddress: "203.0.113.5"}
)

func newRequestAdmissionApplication(t *testing.T) *application.RequestAdmissionApplication {
	mockController := gomock.NewController(t)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	accessTokenProxy.EXPECT().UserIdentifiedBy("xiaoming-token").Return(uint(1), nil).AnyTimes()
	accessTokenProxy.EXPECT().UserIdentifiedBy("xiaohua-token").Return(uint(2), nil).AnyTimes()
	accessTokenProxy.EXPECT().UserIdentifiedBy("expired-token").
		Return(uint(0), domains.ErrAuthenticationRequired).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(admissionMoment).AnyTimes()

	return application.NewRequestAdmissionApplication(service.NewRequestAdmissionService(
		accessTokenProxy,
		clockProxy,
		vo.RequestBudgetVo{RequestsPerMinute: 600, Burst: 120},
		vo.RequestBudgetVo{RequestsPerMinute: 10, Burst: 10},
		vo.LiveStreamCapacityVo{PerRequester: 20, Total: 1000},
	))
}

func TestRequestAdmissionCountsEachRequestAgainstTheRightRequester(t *testing.T) {
	testCases := []struct {
		name               string
		spendGeneral       []dto.RequesterDto
		spendCredential    []dto.RequesterDto
		admit              func(*application.RequestAdmissionApplication) error
		expectedRetryAfter time.Duration
	}{
		{
			name:         "two users behind one address each have their own allowance",
			spendGeneral: repeatedRequester(xiaomingRequester, 120),
			admit:        admitRequest(xiaohuaRequester),
		},
		{
			name:               "an expired token counts against the address",
			spendGeneral:       repeatedRequester(anonymousAtA, 120),
			admit:              admitRequest(expiredRequester),
			expectedRetryAfter: 100 * time.Millisecond,
		},
		{
			name:               "a valid token buys no fresh credential allowance",
			spendCredential:    repeatedRequester(anonymousAtA, 10),
			admit:              admitCredentialRequest(xiaomingAtA),
			expectedRetryAfter: 6 * time.Second,
		},
		{
			name:            "the credential allowance running out leaves the general one alone",
			spendCredential: repeatedRequester(anonymousAtA, 10),
			admit:           admitRequest(anonymousAtA),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestAdmissionApplication := newRequestAdmissionApplication(t)
			for _, requester := range testCase.spendGeneral {
				require.NoError(t, requestAdmissionApplication.AdmitRequest(requester))
			}
			for _, requester := range testCase.spendCredential {
				require.NoError(t, requestAdmissionApplication.AdmitCredentialRequest(requester))
			}

			admissionError := testCase.admit(requestAdmissionApplication)

			if testCase.expectedRetryAfter == 0 {
				assert.NoError(t, admissionError)
				return
			}
			var rateExceeded domains.RequestRateExceededError
			require.True(t, errors.As(admissionError, &rateExceeded))
			assert.Equal(t, testCase.expectedRetryAfter, rateExceeded.RetryAfter)
		})
	}
}

func TestRequestAdmissionHoldsALiveStreamPlaceUntilItCloses(t *testing.T) {
	testCases := []struct {
		name          string
		closeOne      bool
		expectedError error
	}{
		{name: "the twenty-first stream is refused", expectedError: domains.ErrLiveStreamCapacityReached},
		{name: "closing one makes room for another", closeOne: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestAdmissionApplication := newRequestAdmissionApplication(t)
			openedSlots := make([]dto.LiveStreamSlotDto, 0, 20)
			for range 20 {
				liveStreamSlot, openError := requestAdmissionApplication.OpenLiveStream(xiaomingRequester)
				require.NoError(t, openError)
				openedSlots = append(openedSlots, liveStreamSlot)
			}
			if testCase.closeOne {
				requestAdmissionApplication.CloseLiveStream(openedSlots[0])
			}

			_, openError := requestAdmissionApplication.OpenLiveStream(xiaomingAtA)

			assert.Equal(t, testCase.expectedError, openError)
		})
	}
}

func admitRequest(requester dto.RequesterDto) func(*application.RequestAdmissionApplication) error {
	return func(requestAdmissionApplication *application.RequestAdmissionApplication) error {
		return requestAdmissionApplication.AdmitRequest(requester)
	}
}

func admitCredentialRequest(requester dto.RequesterDto) func(*application.RequestAdmissionApplication) error {
	return func(requestAdmissionApplication *application.RequestAdmissionApplication) error {
		return requestAdmissionApplication.AdmitCredentialRequest(requester)
	}
}

func repeatedRequester(requester dto.RequesterDto, count int) []dto.RequesterDto {
	requesters := make([]dto.RequesterDto, 0, count)
	for range count {
		requesters = append(requesters, requester)
	}

	return requesters
}
