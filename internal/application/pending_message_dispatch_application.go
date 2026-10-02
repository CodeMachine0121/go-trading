package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// PendingMessageDispatchApplication sends what bots have queued; every replica runs it, and the queue makes sure each message is taken by one at a time.
type PendingMessageDispatchApplication struct {
	pendingMessageService *service.PendingMessageService
}

func NewPendingMessageDispatchApplication(
	pendingMessageService *service.PendingMessageService,
) *PendingMessageDispatchApplication {
	return &PendingMessageDispatchApplication{pendingMessageService: pendingMessageService}
}

func (pendingMessageDispatchApplication *PendingMessageDispatchApplication) DispatchPendingMessages(
	executionContext context.Context,
) (int, error) {
	return pendingMessageDispatchApplication.pendingMessageService.DispatchPendingMessages(executionContext)
}
