package domains

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// testMessageMaximumLength is Telegram's single-message limit; longer messages are dropped, not truncated.
const testMessageMaximumLength = 4096

type TestMessageDomain struct {
	value string
}

// NewTestMessageDomain refuses rather than truncates an overlong message so the sender doesn't believe it all arrived.
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

func (testMessageDomain TestMessageDomain) Value() string {
	return testMessageDomain.value
}
