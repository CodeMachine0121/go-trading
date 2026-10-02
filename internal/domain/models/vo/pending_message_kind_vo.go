package vo

// PendingMessageKindVo tells a round's message from a bot's message about itself, because only the first carries a signal the bot must forget if it is never delivered.
type PendingMessageKindVo string

const (
	PendingMessageRound     PendingMessageKindVo = "round"
	PendingMessageLifecycle PendingMessageKindVo = "lifecycle"
)
