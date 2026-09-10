package vo_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

// Whether a message arrived is worked out from the reason rather than carried
// beside it, so the two can never disagree — and the disagreement that matters is
// "it failed, and here is no reason why".
func TestDeliveryFailureReasonVoToDto(t *testing.T) {
	testCases := []struct {
		name              string
		reason            vo.DeliveryFailureReasonVo
		expectedDelivered bool
		expectedReason    string
	}{
		{
			name:              "no reason at all means it arrived",
			reason:            vo.DeliveryFailureNone,
			expectedDelivered: true,
			expectedReason:    "",
		},
		{
			name:              "a rejected token did not arrive",
			reason:            vo.DeliveryFailureCredentialRejected,
			expectedDelivered: false,
			expectedReason:    "credentialRejected",
		},
		{
			name:              "an unknown chat did not arrive",
			reason:            vo.DeliveryFailureDestinationNotFound,
			expectedDelivered: false,
			expectedReason:    "destinationNotFound",
		},
		{
			name:              "an unreachable service did not arrive",
			reason:            vo.DeliveryFailureUnreachable,
			expectedDelivered: false,
			expectedReason:    "unreachable",
		},
		{
			name:              "running out of time did not arrive",
			reason:            vo.DeliveryFailureTimedOut,
			expectedDelivered: false,
			expectedReason:    "timedOut",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := testCase.reason.ToDto()

			assert.Equal(t, testCase.expectedDelivered, result.Delivered)
			assert.Equal(t, testCase.expectedReason, result.FailureReason)
		})
	}
}

// Four reasons that read the same are one reason with four names, and somebody with
// a mistyped chat identifier would spend the afternoon replacing a working token.
func TestDeliveryFailureReasonVoKeepsTheFourApart(t *testing.T) {
	reasons := []vo.DeliveryFailureReasonVo{
		vo.DeliveryFailureCredentialRejected,
		vo.DeliveryFailureDestinationNotFound,
		vo.DeliveryFailureUnreachable,
		vo.DeliveryFailureTimedOut,
	}

	seen := map[string]bool{}
	for _, reason := range reasons {
		spelling := reason.ToDto().FailureReason
		assert.NotEmpty(t, spelling, "每一種原因都得說得出自己是哪一種")
		assert.False(t, seen[spelling], "四種原因不得共用同一個取值：%s", spelling)
		seen[spelling] = true
	}
}
