package domains

import (
	"errors"
	"fmt"
)

var ErrAssistantPendingRevisionNotFound = errors.New("assistant pending revision not found")

// ErrAssistantPendingRevisionResolved marks a proposal already confirmed or rejected.
var ErrAssistantPendingRevisionResolved = errors.New("assistant pending revision already resolved")

// ErrAssistantPendingRevisionStale marks a proposal made against a version of its subject that has since changed.
var ErrAssistantPendingRevisionStale = errors.New("assistant pending revision is stale")

// AssistantRevisionProposedNotice is what the assistant reads instead of the rewritten subject, so it cannot
// mistake a proposal for a change.
const AssistantRevisionProposedNotice = "已提出修改，等使用者確認，尚未生效。在使用者確認之前，它維持原本的內容；" +
	"請明白告訴使用者這一筆修改要他確認，不要說已經改好了。"

// AssistantPendingRevisionNotFound is also the answer for someone else's, so identifiers reveal no owners.
func AssistantPendingRevisionNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的待確認修改", ErrAssistantPendingRevisionNotFound, id)
}

func AssistantPendingRevisionResolved() error {
	return fmt.Errorf("%w: 這筆修改已經處理過了", ErrAssistantPendingRevisionResolved)
}

func AssistantPendingRevisionStale() error {
	return fmt.Errorf("%w: 提出這筆修改之後，它已經被改過了，請請助手重新提出", ErrAssistantPendingRevisionStale)
}
