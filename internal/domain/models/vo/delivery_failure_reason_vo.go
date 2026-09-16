package vo

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// DeliveryFailureReasonVo is why one message did not arrive.
//
// There are four of them and they stay four, because each asks the person for a
// different next move: replace the token, check the chat identifier, wait a moment,
// wait a moment. Collapsed into one sentence, somebody with a mistyped chat
// identifier would spend the afternoon regenerating a bot token that was never
// wrong.
//
// It is a named value rather than a sentence. Whoever displays it writes their own
// wording and tells the four apart without matching text — text that changes the
// first time somebody rephrases it, silently taking the four apart with it.
type DeliveryFailureReasonVo string

const (
	// DeliveryFailureNone is not a failure. It is what a delivery that went
	// through reports, so that "did it work" and "why not" are one value rather
	// than two that can contradict each other.
	DeliveryFailureNone DeliveryFailureReasonVo = ""
	// DeliveryFailureCredentialRejected is the destination refusing to believe the
	// bot is who it says it is. Only a new token fixes it.
	DeliveryFailureCredentialRejected DeliveryFailureReasonVo = "credentialRejected"
	// DeliveryFailureDestinationNotFound is the destination not knowing the chat.
	// Only a different chat identifier fixes it.
	DeliveryFailureDestinationNotFound DeliveryFailureReasonVo = "destinationNotFound"
	// DeliveryFailureUnreachable is no answer at all, or an answer nothing here
	// understands. Nothing the person typed fixes it.
	DeliveryFailureUnreachable DeliveryFailureReasonVo = "unreachable"
	// DeliveryFailureTimedOut is an answer that never came in time. It is kept
	// apart from unreachable because "it is slow today" and "it is not there" lead
	// somebody to wait different lengths of time before worrying.
	DeliveryFailureTimedOut DeliveryFailureReasonVo = "timedOut"
)

// ToDto is this reason as the result of one attempt to send.
//
// Whether the message arrived is not carried alongside the reason but worked out
// from it, because two fields saying one thing are two fields that can disagree —
// and the pair that disagrees is "it failed, and here is no reason why".
func (deliveryFailureReasonVo DeliveryFailureReasonVo) ToDto() dto.TestMessageResultDto {
	return dto.TestMessageResultDto{
		Delivered:     deliveryFailureReasonVo == DeliveryFailureNone,
		FailureReason: string(deliveryFailureReasonVo),
	}
}
