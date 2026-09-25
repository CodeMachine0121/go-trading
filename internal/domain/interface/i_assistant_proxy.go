package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_assistant_proxy.go -destination=mocks/mock_i_assistant_proxy.go -package=mocks

// IAssistantProxy makes one stateless round trip to the assistant; the tool loop and budgets stay in the domain so they are testable without a real assistant.
type IAssistantProxy interface {
	// Reply answers or asks for capabilities to run first; unreachable, timed-out or empty replies are errors.
	Reply(executionContext context.Context, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error)
}
