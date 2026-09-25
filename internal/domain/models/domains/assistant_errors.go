package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrAssistantAskEmpty marks a question that is empty or blanks only.
var ErrAssistantAskEmpty = errors.New("assistant ask is empty")

var ErrConversationNotFound = errors.New("conversation not found")

// ErrDailyUsageAllowanceExhausted is distinct from validation failures because the question was fine; the reader must wait, not rewrite.
var ErrDailyUsageAllowanceExhausted = errors.New("daily usage allowance exhausted")

// ErrAssistantUnavailable covers an unreachable, too slow or silent assistant, all answered by retrying later.
var ErrAssistantUnavailable = errors.New("assistant unavailable")

// ErrAssistantAnswerInProgress marks a question sent while the conversation's previous answer is still being written.
var ErrAssistantAnswerInProgress = errors.New("assistant answer in progress")

// ErrAssistantQueryArgument is returned to the assistant as a refusal reason, not to the caller as a failure.
var ErrAssistantQueryArgument = errors.New("assistant query argument rejected")

// ConversationNotFound is shared by the store and the service so both give the same message.
func ConversationNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的對話", ErrConversationNotFound, id)
}

// DailyUsageAllowanceExhausted names when the allowance resets.
func DailyUsageAllowanceExhausted(allowance int, resetsAt time.Time) error {
	return fmt.Errorf(
		"%w: 今日助手用量額度 %d 已用盡，於 %s 重置",
		ErrDailyUsageAllowanceExhausted, allowance, resetsAt.Format(time.RFC3339))
}

// AssistantUnavailable wraps the cause so timeouts and outages stay distinguishable in logs.
func AssistantUnavailable(cause error) error {
	return fmt.Errorf("%w: 助手目前沒有回應，請稍後再試: %w", ErrAssistantUnavailable, cause)
}

// AssistantAnsweredNothing treats a blank answer as an unavailable assistant.
func AssistantAnsweredNothing() error {
	return fmt.Errorf("%w: 助手回了空白的答案，請稍後再試", ErrAssistantUnavailable)
}

// AssistantTurnNotFound means the conversation was deleted while the answer was being written, so it reuses ErrConversationNotFound.
func AssistantTurnNotFound(turnID uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的問答", ErrConversationNotFound, turnID)
}

// AssistantBrokeDown closes an answer after an unexpected failure; details go to the log.
func AssistantBrokeDown() error {
	return fmt.Errorf("%w: 這則回答在產生的過程中出錯了，請再問一次", ErrAssistantUnavailable)
}

func AssistantAnswerInProgress() error {
	return fmt.Errorf(
		"%w: 這段對話上還有一則回答正在進行中，請等它結束再問下一句",
		ErrAssistantAnswerInProgress)
}

// AssistantAnswerInterruptedByRestart is written at startup, since shutdown cannot be relied on to clean up running answers.
func AssistantAnswerInterruptedByRestart() string {
	return "系統重新啟動時中斷了這則回答，請再問一次"
}
