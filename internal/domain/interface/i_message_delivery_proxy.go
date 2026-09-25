package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_message_delivery_proxy.go -destination=mocks/mock_i_message_delivery_proxy.go -package=mocks

// IMessageDeliveryProxy sends one message a person can read; Telegram is currently the only implementation.
type IMessageDeliveryProxy interface {
	// Deliver returns a destination's refusal as a reason with a nil error; only local failures (unbuildable request, unreadable answer) are errors.
	Deliver(
		executionContext context.Context,
		credential vo.MessageDeliveryCredentialVo,
		message string,
	) (vo.DeliveryFailureReasonVo, error)
}
