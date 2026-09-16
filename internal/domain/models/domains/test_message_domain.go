package domains

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// testMessageMaximumLength is how many characters one message may carry.
//
// It is not this system's preference. It is what a single Telegram message holds,
// and a longer one does not arrive truncated — it does not arrive. The number lives
// here, in the rules, rather than inside the code that talks to Telegram, because
// whoever pasted too much has to be told so, and only the rules get to say things to
// people.
const testMessageMaximumLength = 4096

// TestMessageDomain holds one message meant to prove the route to somebody's
// Telegram works, and guarantees it is something that can actually be sent.
type TestMessageDomain struct {
	value string
}

// NewTestMessageDomain trims the message and judges what is left.
//
// Being too long is a refusal rather than a trim, for the same reason a password
// that is too long is refused: somebody who sent four thousand characters and was
// told it worked would believe all four thousand arrived.
func NewTestMessageDomain(message string) (TestMessageDomain, error) {
	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" {
		return TestMessageDomain{}, fmt.Errorf(
			"%w: 訊息不得為空白", ErrTelegramDeliveryValidation)
	}

	if utf8.RuneCountInString(trimmedMessage) > testMessageMaximumLength {
		return TestMessageDomain{}, fmt.Errorf(
			"%w: 一則訊息上限為 %d 個字元", ErrTelegramDeliveryValidation, testMessageMaximumLength)
	}

	return TestMessageDomain{value: trimmedMessage}, nil
}

// Value is the message as it will be sent.
func (testMessageDomain TestMessageDomain) Value() string {
	return testMessageDomain.value
}
