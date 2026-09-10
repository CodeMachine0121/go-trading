package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_message_delivery_proxy.go -destination=mocks/mock_i_message_delivery_proxy.go -package=mocks

// IMessageDeliveryProxy sends one message somewhere a person can read it.
//
// It is named for the capability and not for Telegram, although Telegram is the
// only implementation today. Which service carries a message is exactly the kind of
// thing that gets replaced or joined by a second one, and an interface named after
// the first of them stops being an abstraction the moment the second arrives.
//
// It knows nothing about test messages. Sending a message somebody typed and sending
// one the system generated are the same act, and the day trading signals go out this
// way, none of this changes.
type IMessageDeliveryProxy interface {
	// Deliver sends the message and reports how it went.
	//
	// A destination that refuses comes back as a reason with a nil error, because
	// asking and being told no is a question that was successfully answered. Only
	// this side failing — a request that cannot be built, an answer that cannot be
	// read — is an error. Written the other way, every caller would have to sort
	// four expected outcomes back out of a pile of errors.
	Deliver(
		executionContext context.Context,
		credential vo.MessageDeliveryCredentialVo,
		message string,
	) (vo.DeliveryFailureReasonVo, error)
}
