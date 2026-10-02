package vo

import "time"

// DeliveryResultVo is how one send went; RetryAfter is set only when the destination said how long to wait before trying again.
type DeliveryResultVo struct {
	FailureReason DeliveryFailureReasonVo
	RetryAfter    time.Duration
}
