package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// PendingMessageQueueDomain keeps each person's messages in the order they were queued: only a person's oldest unsettled message may be sent.
type PendingMessageQueueDomain struct {
	messages []entities.PendingMessage
}

// NewPendingMessageQueueDomain expects the messages oldest first.
func NewPendingMessageQueueDomain(messages []entities.PendingMessage) PendingMessageQueueDomain {
	return PendingMessageQueueDomain{messages: messages}
}

// DueAt is each person's oldest unsettled message when it may be sent now, plus any later one that has already expired:
// an expired message is never sent, so giving it up cannot break the order, and holding it back would keep its bot believing a signal arrived.
func (pendingMessageQueueDomain PendingMessageQueueDomain) DueAt(now time.Time) []PendingMessageDomain {
	reachedRecipients := map[uint]bool{}
	due := []PendingMessageDomain{}

	for _, message := range pendingMessageQueueDomain.messages {
		candidate := NewPendingMessageDomain(message)
		isHead := !reachedRecipients[message.RecipientUserID]
		reachedRecipients[message.RecipientUserID] = true

		if !candidate.IsDispatchableAt(now) {
			continue
		}
		if isHead || candidate.IsExpiredAt(now) {
			due = append(due, candidate)
		}
	}

	return due
}
