package vo

// PendingMessageAbandonReasonVo is why a message will never be sent: it came too late to matter, or its destination refused it for good.
type PendingMessageAbandonReasonVo string

const (
	PendingMessageAbandonedExpired               PendingMessageAbandonReasonVo = "expired"
	PendingMessageAbandonedCredentialRejected    PendingMessageAbandonReasonVo = "credentialRejected"
	PendingMessageAbandonedDestinationNotFound   PendingMessageAbandonReasonVo = "destinationNotFound"
	PendingMessageAbandonedDeliveryNotConfigured PendingMessageAbandonReasonVo = "deliveryNotConfigured"
)
