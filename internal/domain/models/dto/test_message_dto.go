package dto

// TestMessageDto is one message somebody wants sent to their own Telegram to find
// out whether the route works at all.
//
// It is a single field wrapped in a type rather than a bare string, because what a
// service is handed should say what it is. A second thing to send along — a subject,
// a way of formatting it — becomes a field here rather than a second parameter every
// caller has to be changed to pass.
type TestMessageDto struct {
	Message string
}
