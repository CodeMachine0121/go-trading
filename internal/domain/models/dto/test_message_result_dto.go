package dto

// TestMessageResultDto is what came of trying to send one test message.
//
// A message Telegram would not accept is reported here rather than as a failure of
// the request, because the request did exactly what it was asked to: it tried, and
// it found out. Calling that a broken system would be untrue, and it would leave
// nowhere to say which of the four things went wrong.
//
// FailureReason is one of the named delivery failure reasons, carried as text the
// way every other enum leaves this layer. It is empty when the message went through.
type TestMessageResultDto struct {
	Delivered     bool   `json:"delivered"`
	FailureReason string `json:"failureReason,omitempty"`
}
