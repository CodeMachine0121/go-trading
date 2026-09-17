package service

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// assistantAnswerWriter is one answer being written: which exchange it is filling in,
// on whose behalf, and how far the conversation with the assistant has got.
//
// It is not a domain model and does not live with them. What it holds is execution —
// a place in storage to write back to and a conversation with an outside service in
// progress — and taking those away leaves no domain concept behind. Every rule it
// obeys still belongs to AssistantExchangeDomain; this only drives it and records
// where it ended up.
//
// It exists at all because of one requirement: the answer must outlive the request
// that asked for it. Somebody closing a tab must not kill work that has already built
// two strategy scripts and replayed them three times, and a browser refresh must find
// the answer still being written rather than nothing at all.
type assistantAnswerWriter struct {
	assistantConversationService *AssistantConversationService
	viewerID                     uint
	// turnID is the exchange this is filling in. It was written before any of this
	// started, which is what makes the work findable while it is still going.
	turnID   uint
	exchange domains.AssistantExchangeDomain
}

// write drives the round trips to an end and records it, either way.
//
// **It takes no context from the caller and this is the whole point.** Given the
// request's context, the answer would be cancelled the instant the asker's connection
// went away — which is exactly the case this was built for. What bounds it instead is
// the assistant's own per-round-trip timeout, and what cleans up after a shutdown is
// the sweep at startup.
//
// Neither ending returns anything. There is nobody left to return to: the question
// was answered with a place to look, and this is what fills that place in.
func (assistantAnswerWriter assistantAnswerWriter) write() {
	executionContext := context.Background()

	// **A panic here used to take one request down; now it would take the process.**
	// Until this loop moved off the request, the HTTP layer's own recovery contained
	// it to a single failed answer. Out here nothing is above it, so a panic anywhere
	// in up to forty rounds of tool calls — a replay engine, a proxy, a decimal
	// division — would stop the API, the background jobs, and every other answer
	// being written at that moment.
	//
	// The exchange is closed as failed on the way out, so the row does not sit at
	// running until the next restart sweeps it. What is recorded is deliberately the
	// same sentence any other breakage gets: the panic value is for the log, and the
	// person waiting can do exactly one thing about it either way.
	defer func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}

		log.Printf("assistant answer %d panicked: %v\n%s",
			assistantAnswerWriter.turnID, panicValue, debug.Stack())

		assistantAnswerWriter.recordEnding(
			executionContext,
			assistantAnswerWriter.exchange.ToFailedTurn(
				assistantAnswerWriter.turnID, domains.AssistantBrokeDown().Error()))
	}()

	answeredExchange, answer, exchangeError := assistantAnswerWriter.assistantConversationService.writeAnswer(
		executionContext, assistantAnswerWriter.viewerID, assistantAnswerWriter.exchange)

	if exchangeError != nil {
		// The reason is recorded as the assistant's own sentence, because that
		// sentence is written to be read by whoever asked — it already says to try
		// again later, in their language.
		assistantAnswerWriter.recordEnding(
			executionContext,
			answeredExchange.ToFailedTurn(assistantAnswerWriter.turnID, exchangeError.Error()))

		return
	}

	assistantAnswerWriter.recordEnding(
		executionContext,
		answeredExchange.ToAnsweredTurn(assistantAnswerWriter.turnID, answer))
}

// recordEnding writes how this ended, and swallows a failure to write it.
//
// There is nowhere for that failure to go. Nobody is waiting on this call, and the
// one row that could carry the news is the very row that would not save. What catches
// it instead is the sweep at startup: an exchange left at running is picked up and
// reported as interrupted, which is a truthful enough account of a system that could
// not write to its own store.
func (assistantAnswerWriter assistantAnswerWriter) recordEnding(
	executionContext context.Context, turn entities.AssistantTurn,
) {
	_ = assistantAnswerWriter.assistantConversationService.conversationRepository.CompleteTurn(
		executionContext, turn)
}
