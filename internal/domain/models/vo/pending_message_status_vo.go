package vo

// PendingMessageStatusVo is where one message waiting to be sent stands; sent and abandoned are settled and never change again.
type PendingMessageStatusVo string

const (
	// PendingMessageReady may be taken by any replica once its next attempt is due.
	PendingMessageReady PendingMessageStatusVo = "ready"
	// PendingMessageSending has been taken by one replica; it becomes takeable again if that replica's claim runs out.
	PendingMessageSending PendingMessageStatusVo = "sending"
	PendingMessageSent    PendingMessageStatusVo = "sent"
	// PendingMessageAbandoned will never be sent: it expired, its destination refused for good, or nobody could be told.
	PendingMessageAbandoned PendingMessageStatusVo = "abandoned"
)
