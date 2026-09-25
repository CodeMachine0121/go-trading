package dto

// TestMessageResultDto reports a Telegram rejection as a result, not a request failure;
// FailureReason is empty on success.
type TestMessageResultDto struct {
	Delivered     bool   `json:"delivered"`
	FailureReason string `json:"failureReason,omitempty"`
}
