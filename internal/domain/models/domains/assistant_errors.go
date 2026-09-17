package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrAssistantAskEmpty marks a question with nothing in it. Blanks only is the same
// thing as nothing: whoever sent it did not ask anything, and answering it would mean
// paying an assistant to guess what was meant.
var ErrAssistantAskEmpty = errors.New("assistant ask is empty")

// ErrConversationNotFound marks a conversation named by an identifier that does not
// exist.
var ErrConversationNotFound = errors.New("conversation not found")

// ErrDailyUsageAllowanceExhausted marks a question refused because today's allowance
// is spent. It is deliberately its own refusal rather than a validation failure: the
// question was fine, and what the reader has to do about it is wait, not rewrite.
var ErrDailyUsageAllowanceExhausted = errors.New("daily usage allowance exhausted")

// ErrAssistantUnavailable marks an assistant that did not answer — unreachable,
// too slow, or silent. All three leave the same nothing behind, and all three are
// answered by trying again later.
var ErrAssistantUnavailable = errors.New("assistant unavailable")

// ErrAssistantAnswerInProgress marks a question sent to a conversation whose previous
// answer is still being written.
//
// It is its own refusal rather than one of the others because what the reader has to
// do about it is unlike all of them: not rewrite, not wait until tomorrow, not try a
// different assistant — just wait a moment for the one already running.
var ErrAssistantAnswerInProgress = errors.New("assistant answer in progress")

// ErrAssistantQueryArgument marks an assistant query whose arguments broke a rule.
// It travels back to the assistant as the reason, not up to the caller as a failure:
// the assistant asked for something it may not have, and asking differently is
// within its power.
var ErrAssistantQueryArgument = errors.New("assistant query argument rejected")

// ConversationNotFound is the refusal a reader gets when no conversation carries this
// identifier. Both the store and the service arrive at it, and both owe the reader
// the same sentence.
func ConversationNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的對話", ErrConversationNotFound, id)
}

// DailyUsageAllowanceExhausted is the refusal a reader gets once today's allowance is
// spent. It names when the allowance comes back, because a refusal that does not say
// how long to wait leaves nothing to do but keep trying.
func DailyUsageAllowanceExhausted(allowance int, resetsAt time.Time) error {
	return fmt.Errorf(
		"%w: 今日助手用量額度 %d 已用盡，於 %s 重置",
		ErrDailyUsageAllowanceExhausted, allowance, resetsAt.Format(time.RFC3339))
}

// AssistantUnavailable is the refusal a reader gets when the assistant did not
// answer. The underlying cause is wrapped rather than described, so that a timeout
// and an unreachable service stay distinguishable to whoever is debugging while
// reading the same way to whoever is asking.
func AssistantUnavailable(cause error) error {
	return fmt.Errorf("%w: 助手目前沒有回應，請稍後再試: %w", ErrAssistantUnavailable, cause)
}

// AssistantAnsweredNothing is the refusal a reader gets when the assistant came back
// with a blank answer. It is the same refusal as an assistant that never answered,
// because a blank answer is not an answer.
func AssistantAnsweredNothing() error {
	return fmt.Errorf("%w: 助手回了空白的答案，請稍後再試", ErrAssistantUnavailable)
}

// AssistantAnswerInProgress is the refusal a reader gets for asking again while the
// previous answer on that conversation is still being written.
func AssistantAnswerInProgress() error {
	return fmt.Errorf(
		"%w: 這段對話上還有一則回答正在進行中，請等它結束再問下一句",
		ErrAssistantAnswerInProgress)
}

// AssistantAnswerInterruptedByRestart is what is left on an answer the system was
// still writing when it was shut down.
//
// It is written on the way up rather than on the way down, because a shutdown is not
// always given the chance to tidy up — and an answer stuck at running forever is a
// wait nobody can end and a conversation nobody can ask anything else.
func AssistantAnswerInterruptedByRestart() string {
	return "系統重新啟動時中斷了這則回答，請再問一次"
}
