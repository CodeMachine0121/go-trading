package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantConversationService is the application layer's only entry point to the assistant; reading conversations back never touches the assistant or the daily allowance.
type AssistantConversationService struct {
	conversationRepository domaininterface.IConversationRepository
	assistantProxy         domaininterface.IAssistantProxy
	assistantQueries       []domaininterface.IAssistantQuery
	clockProxy             domaininterface.IClockProxy
	// declarations are computed once so the declared and reachable capability lists are the same.
	declarations        []vo.AssistantQueryDeclarationVo
	recentMessageLimit  int
	queryLimit          int
	dailyUsageAllowance int
	answerLengthLimit   int
}

func NewAssistantConversationService(
	conversationRepository domaininterface.IConversationRepository,
	assistantProxy domaininterface.IAssistantProxy,
	assistantQueries []domaininterface.IAssistantQuery,
	clockProxy domaininterface.IClockProxy,
	recentMessageLimit int,
	queryLimit int,
	dailyUsageAllowance int,
	answerLengthLimit int,
) *AssistantConversationService {
	declarations := make([]vo.AssistantQueryDeclarationVo, 0, len(assistantQueries))
	for _, assistantQuery := range assistantQueries {
		declarations = append(declarations, vo.AssistantQueryDeclarationVo{
			Name:           assistantQuery.Name(),
			Description:    assistantQuery.Description(),
			ArgumentSchema: assistantQuery.ArgumentSchema(),
		})
	}

	return &AssistantConversationService{
		conversationRepository: conversationRepository,
		assistantProxy:         assistantProxy,
		assistantQueries:       assistantQueries,
		clockProxy:             clockProxy,
		declarations:           declarations,
		recentMessageLimit:     recentMessageLimit,
		queryLimit:             queryLimit,
		dailyUsageAllowance:    dailyUsageAllowance,
		answerLengthLimit:      answerLengthLimit,
	}
}

// Ask validates the question, reserves the answer's place, and writes the answer in the background, returning where it will appear; checks run cheapest first and nothing is stored until all pass.
func (assistantConversationService *AssistantConversationService) Ask(
	executionContext context.Context, askDto dto.AssistantAskDto,
) (dto.AssistantAnswerStartedDto, error) {
	ask, askError := domains.NewAssistantAskDomain(askDto.Question)
	if askError != nil {
		return dto.AssistantAnswerStartedDto{}, askError
	}

	now := assistantConversationService.clockProxy.Now()
	allowance := domains.NewDailyUsageAllowanceDomain(
		assistantConversationService.dailyUsageAllowance, now)

	usageToday, sumError := assistantConversationService.conversationRepository.SumUsageBetween(
		executionContext, allowance.StartOfDay(), allowance.ResetsAt())
	if sumError != nil {
		return dto.AssistantAnswerStartedDto{}, sumError
	}

	if allowance.Exhausted(usageToday) {
		return dto.AssistantAnswerStartedDto{}, domains.DailyUsageAllowanceExhausted(
			allowance.Allowance(), allowance.ResetsAt())
	}

	recentMessages, recentMessagesError := assistantConversationService.recentMessagesOf(
		executionContext, askDto.ViewerID, askDto.ConversationID)
	if recentMessagesError != nil {
		return dto.AssistantAnswerStartedDto{}, recentMessagesError
	}

	exchange := domains.NewAssistantExchangeDomain(
		ask.Question(),
		recentMessages,
		assistantConversationService.declarations,
		assistantConversationService.queryLimit,
		assistantConversationService.answerLengthLimit,
	)

	conversationID, turnID, startError := assistantConversationService.start(
		executionContext, askDto.ViewerID, askDto.ConversationID, exchange.ToStartedTurn(now))
	if startError != nil {
		return dto.AssistantAnswerStartedDto{}, startError
	}

	go assistantAnswerWriter{
		assistantConversationService: assistantConversationService,
		viewerID:                     askDto.ViewerID,
		turnID:                       turnID,
		exchange:                     exchange,
	}.write()

	return dto.AssistantAnswerStartedDto{
		ConversationID: conversationID,
		TurnID:         turnID,
		Status:         string(vo.AssistantTurnRunning),
	}, nil
}

// ListConversations returns this person's conversations, most recently active first; ownership is filtered by the store.
func (assistantConversationService *AssistantConversationService) ListConversations(
	executionContext context.Context, viewerID uint,
) ([]dto.ConversationSummaryDto, error) {
	conversations, findError := assistantConversationService.conversationRepository.FindAllOwnedBy(
		executionContext, viewerID)
	if findError != nil {
		return nil, findError
	}

	summaryDtos := make([]dto.ConversationSummaryDto, 0, len(conversations))
	for _, conversation := range conversations {
		summaryDtos = append(summaryDtos, domains.NewConversationDomain(conversation).ToSummaryDto())
	}

	return summaryDtos, nil
}

// GetConversation returns one of this person's conversations with every message; someone else's reads as not found because transcripts can contain their algorithms.
func (assistantConversationService *AssistantConversationService) GetConversation(
	executionContext context.Context, viewerID uint, id uint,
) (dto.ConversationDto, error) {
	conversation, findError := assistantConversationService.conversationRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.ConversationDto{}, findError
	}

	conversationDomain := domains.NewConversationDomain(conversation)
	if ownershipError := conversationDomain.RequireOwnership(viewerID); ownershipError != nil {
		return dto.ConversationDto{}, ownershipError
	}

	return conversationDomain.ToDto(), nil
}

// recentMessagesOf returns what the assistant may remember of the question's conversation, and is the single place that rejects someone else's conversation or one with an answer still in flight.
func (assistantConversationService *AssistantConversationService) recentMessagesOf(
	executionContext context.Context, viewerID uint, conversationId uint,
) ([]vo.AssistantMessageVo, error) {
	if conversationId == 0 {
		return make([]vo.AssistantMessageVo, 0), nil
	}

	conversation, findError := assistantConversationService.conversationRepository.FindOne(
		executionContext, conversationId)
	if findError != nil {
		return nil, findError
	}

	conversationDomain := domains.NewConversationDomain(conversation)
	if ownershipError := conversationDomain.RequireOwnership(viewerID); ownershipError != nil {
		return nil, ownershipError
	}

	// Checked after ownership so probing a stranger's conversation reads as not found rather than busy.
	if conversationDomain.HasAnswerInFlight() {
		return nil, domains.AssistantAnswerInProgress()
	}

	return conversationDomain.RecentMessages(
		assistantConversationService.recentMessageLimit), nil
}

// writeAnswer loops round trips until an answer; lookups are checked before text because the assistant often narrates alongside a lookup, and an empty reply after the query limit counts as no answer.
func (assistantConversationService *AssistantConversationService) writeAnswer(
	executionContext context.Context, viewerID uint, exchange domains.AssistantExchangeDomain,
) (domains.AssistantExchangeDomain, string, error) {
	for {
		reply, replyError := assistantConversationService.assistantProxy.Reply(
			executionContext, exchange.Request())
		if replyError != nil {
			return exchange, "", domains.AssistantUnavailable(replyError)
		}

		exchange = exchange.RecordUsage(reply.Usage)

		// 先檢查查詢請求再看回答，否則「我先看一下…」會被當成答案而提前結束。
		allowedCalls := exchange.AllowedCalls(reply.QueryCalls)
		if len(allowedCalls) > 0 {
			exchange = exchange.RecordRound(
				reply.Answer,
				assistantConversationService.runAssistantQueries(executionContext, viewerID, allowedCalls))

			continue
		}

		if reply.Answer != "" {
			return exchange, reply.Answer, nil
		}

		return exchange, "", domains.AssistantAnsweredNothing()
	}
}

// runAssistantQueries runs lookups in the order asked; every request gets a result, refusals included, as the assistant's interface requires.
func (assistantConversationService *AssistantConversationService) runAssistantQueries(
	executionContext context.Context, viewerID uint, calls []vo.AssistantQueryCallVo,
) []vo.AssistantQueryExchangeVo {
	exchanges := make([]vo.AssistantQueryExchangeVo, 0, len(calls))
	for _, call := range calls {
		outcome, rejected := assistantConversationService.runAssistantQuery(executionContext, viewerID, call)
		exchanges = append(exchanges, vo.AssistantQueryExchangeVo{
			Call:     call,
			Outcome:  outcome,
			Rejected: rejected,
		})
	}

	return exchanges
}

// runAssistantQuery turns refusals into data for the assistant rather than failures, so one bad lookup does not discard the ones that succeeded.
func (assistantConversationService *AssistantConversationService) runAssistantQuery(
	executionContext context.Context, viewerID uint, call vo.AssistantQueryCallVo,
) (string, bool) {
	for _, assistantQuery := range assistantConversationService.assistantQueries {
		if assistantQuery.Name() != call.Name {
			continue
		}

		outcome, runError := assistantQuery.Run(executionContext, viewerID, call.Arguments)
		if runError != nil {
			return domains.AssistantReadableReason(runError), true
		}

		return outcome, false
	}

	return "系統沒有「" + call.Name + "」這個能力。請改用已提供的能力，或告知使用者這件事辦不到。", true
}

// start reserves the answer's place in a single statement (new conversation or appended exchange) and uses the store-assigned exchange ID, avoiding races; it runs on the asker's context.
func (assistantConversationService *AssistantConversationService) start(
	executionContext context.Context, viewerID uint, conversationId uint, turn entities.AssistantTurn,
) (uint, uint, error) {
	if conversationId != 0 {
		appendedTurn, appendError := assistantConversationService.conversationRepository.AppendTurn(
			executionContext, conversationId, turn)
		if appendError != nil {
			return 0, 0, appendError
		}

		return conversationId, appendedTurn.ID, nil
	}

	startedConversation, saveError := assistantConversationService.conversationRepository.Save(
		executionContext,
		entities.Conversation{
			OwnerID:      viewerID,
			LastActiveAt: turn.CreatedAt,
			Turns:        []entities.AssistantTurn{turn},
		})
	if saveError != nil {
		return 0, 0, saveError
	}

	return startedConversation.ID, startedConversation.Turns[0].ID, nil
}

// FailInterruptedAnswers marks answers left running by the last shutdown as failed and returns how many.
func (assistantConversationService *AssistantConversationService) FailInterruptedAnswers(
	executionContext context.Context,
) (int, error) {
	return assistantConversationService.conversationRepository.FailAllRunningTurns(
		executionContext, domains.AssistantAnswerInterruptedByRestart())
}
