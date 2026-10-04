package vo

// PendingMessageKindVo tells a round's message from a bot's message about itself, because only the first carries a signal the bot must forget if it is never delivered.
type PendingMessageKindVo string

const (
	PendingMessageRound     PendingMessageKindVo = "round"
	PendingMessageLifecycle PendingMessageKindVo = "lifecycle"
	// PendingMessageAutoOrder reports what an auto order did; it speaks for no round, so it never takes a round's place in the queue.
	PendingMessageAutoOrder PendingMessageKindVo = "autoOrder"
)
