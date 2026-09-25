package service

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// assistantAnswerWriter drives one answer to completion off the request, so a closed tab or refresh does not lose work in progress.
type assistantAnswerWriter struct {
	assistantConversationService *AssistantConversationService
	viewerID                     uint
	turnID                       uint
	exchange                     domains.AssistantExchangeDomain
}

// write deliberately ignores the caller's context so a dropped connection cannot cancel the answer; per-round-trip timeouts bound it and the startup sweep cleans up after shutdowns.
func (assistantAnswerWriter assistantAnswerWriter) write() {
	executionContext := context.Background()

	// Off the request there is no HTTP recovery, so a panic here would crash the process; recover and close the exchange as failed.
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
		// Record the assistant's own error sentence, which is written for the asker.
		assistantAnswerWriter.recordEnding(
			executionContext,
			answeredExchange.ToFailedTurn(assistantAnswerWriter.turnID, exchangeError.Error()))

		return
	}

	assistantAnswerWriter.recordEnding(
		executionContext,
		answeredExchange.ToAnsweredTurn(assistantAnswerWriter.turnID, answer))
}

// recordEnding swallows write failures; the startup sweep reports an exchange left running as interrupted.
func (assistantAnswerWriter assistantAnswerWriter) recordEnding(
	executionContext context.Context, turn entities.AssistantTurn,
) {
	_ = assistantAnswerWriter.assistantConversationService.conversationRepository.CompleteTurn(
		executionContext, turn)
}
