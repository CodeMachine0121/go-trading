package vo

// AssistantQueryOriginVo says where a lookup was asked from, so capabilities act as the asker and can tie
// what they create or propose to the conversation and answer it came out of.
type AssistantQueryOriginVo struct {
	ViewerID       uint
	ConversationID uint
	TurnID         uint
}
