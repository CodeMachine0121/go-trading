package vo

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// DeliveryFailureReasonVo is why a message did not arrive; the four reasons stay distinct because each calls for a different fix.
type DeliveryFailureReasonVo string

const (
	// DeliveryFailureNone means the delivery succeeded.
	DeliveryFailureNone DeliveryFailureReasonVo = ""
	// DeliveryFailureCredentialRejected means the bot token is wrong.
	DeliveryFailureCredentialRejected DeliveryFailureReasonVo = "credentialRejected"
	// DeliveryFailureDestinationNotFound means the chat identifier is wrong.
	DeliveryFailureDestinationNotFound DeliveryFailureReasonVo = "destinationNotFound"
	// DeliveryFailureUnreachable is no answer or an unintelligible one.
	DeliveryFailureUnreachable DeliveryFailureReasonVo = "unreachable"
	// DeliveryFailureTimedOut is kept apart from unreachable because slow and absent warrant different waits.
	DeliveryFailureTimedOut DeliveryFailureReasonVo = "timedOut"
)

// ToDto derives success from the reason so the two can never disagree.
func (deliveryFailureReasonVo DeliveryFailureReasonVo) ToDto() dto.TestMessageResultDto {
	return dto.TestMessageResultDto{
		Delivered:     deliveryFailureReasonVo == DeliveryFailureNone,
		FailureReason: string(deliveryFailureReasonVo),
	}
}
