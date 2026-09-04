package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_assistant_proxy.go -destination=mocks/mock_i_assistant_proxy.go -package=mocks

// IAssistantProxy asks the assistant once and reports what came back.
//
// One round trip is the whole of it. It runs no capability, keeps no state and never
// loops: hand it the messages and what may be used, and it either answers or says
// which capabilities it wants run first.
//
// That line is where the design earns its keep. How many queries an answer may spend,
// what happens when they run out, how much of a candle series the assistant may see
// and how the day's allowance is settled are all rules about this business, and rules
// belong where they can be tested without an assistant on the other end. Handing the
// whole loop to whichever library the assistant ships with would move every one of
// them into infrastructure, where the only way to check them is to pay for a real
// answer.
//
// It is named by capability, not by supplier: swapping assistants is a new
// implementation, not a new interface.
type IAssistantProxy interface {
	// Reply answers, or asks for capabilities to be run first. An assistant that
	// cannot be reached, takes too long, or says nothing is an error — the caller
	// then leaves no half exchange behind.
	Reply(executionContext context.Context, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error)
}
