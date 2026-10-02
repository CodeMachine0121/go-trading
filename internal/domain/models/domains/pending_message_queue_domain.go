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

// HeadsDispatchableAt is each person's oldest unsettled message, when it may be sent now; a later one waits until the earlier is sent or given up.
func (pendingMessageQueueDomain PendingMessageQueueDomain) HeadsDispatchableAt(now time.Time) []PendingMessageDomain {
	reachedRecipients := map[uint]bool{}
	heads := []PendingMessageDomain{}

	for _, message := range pendingMessageQueueDomain.messages {
		if reachedRecipients[message.RecipientUserID] {
			continue
		}
		reachedRecipients[message.RecipientUserID] = true

		head := NewPendingMessageDomain(message)
		if head.IsDispatchableAt(now) {
			heads = append(heads, head)
		}
	}

	return heads
}
