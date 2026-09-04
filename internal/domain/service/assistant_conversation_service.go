package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantConversationService is the application layer's only entry point for
// talking to the assistant. Its public use-case methods never call one another.
//
// Asking is one call in and one answer out. Whoever calls it does not check the
// day's allowance, does not trim the conversation, does not drive the round trips and
// does not decide when to store anything — all of that is in here, because all of it
// is rules about what an answer may cost, and rules that leak out to callers are
// rules each caller gets slightly wrong.
//
// Reading a conversation back deliberately touches neither the assistant nor the
// allowance. Today's ceiling is a brake on new answers, not on the record of old
// ones, and an assistant that is down must not take the record with it.
type AssistantConversationService struct {
	conversationRepository domaininterface.IConversationRepository
	assistantProxy         domaininterface.IAssistantProxy
	assistantQueries       []domaininterface.IAssistantQuery
	clockProxy             domaininterface.IClockProxy
	// declarations are what the assistant is told it may do. They are worked out once
	// at construction because the set never changes while the system is running, and
	// because the list declared and the list reachable are then the same list by
	// construction.
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

// Ask answers one question and stores the exchange.
//
// The order of the first three checks is the order the refusals cost least in. A
// question with nothing in it is refused before the day's usage is read, and the
// day's usage before the conversation is fetched, so that the cheapest refusal never
// pays for the more expensive one's lookup. It also keeps the refusals honest: a
// blank question sent to a conversation that does not exist is answered as a blank
// question, which is the thing the sender can actually fix.
//
// Nothing is stored until the assistant has answered. That is what makes "an
// assistant that never answered leaves nothing behind" a property of this method
// rather than a cleanup somebody has to remember.
func (assistantConversationService *AssistantConversationService) Ask(
	executionContext context.Context, askDto dto.AssistantAskDto,
) (dto.AssistantAnswerDto, error) {
	ask, askError := domains.NewAssistantAskDomain(askDto.Question)
	if askError != nil {
		return dto.AssistantAnswerDto{}, askError
	}

	now := assistantConversationService.clockProxy.Now()
	allowance := domains.NewDailyUsageAllowanceDomain(
		assistantConversationService.dailyUsageAllowance, now)

	usageToday, sumError := assistantConversationService.conversationRepository.SumUsageBetween(
		executionContext, allowance.StartOfDay(), allowance.ResetsAt())
	if sumError != nil {
		return dto.AssistantAnswerDto{}, sumError
	}

	if allowance.Exhausted(usageToday) {
		return dto.AssistantAnswerDto{}, domains.DailyUsageAllowanceExhausted(
			allowance.Allowance(), allowance.ResetsAt())
	}

	recentMessages, recentMessagesError := assistantConversationService.recentMessagesOf(
		executionContext, askDto.ConversationID)
	if recentMessagesError != nil {
		return dto.AssistantAnswerDto{}, recentMessagesError
	}

	exchange := domains.NewAssistantExchangeDomain(
		ask.Question(),
		recentMessages,
		assistantConversationService.declarations,
		assistantConversationService.queryLimit,
		assistantConversationService.answerLengthLimit,
	)

	answeredExchange, answer, exchangeError := assistantConversationService.writeAnswer(
		executionContext, exchange)
	if exchangeError != nil {
		return dto.AssistantAnswerDto{}, exchangeError
	}

	return assistantConversationService.store(
		executionContext, askDto.ConversationID, answeredExchange.ToTurn(answer, now))
}

// ListConversations returns every conversation, the most recently active first.
// Holding none is an answer rather than a failure.
func (assistantConversationService *AssistantConversationService) ListConversations(
	executionContext context.Context,
) ([]dto.ConversationSummaryDto, error) {
	conversations, findError := assistantConversationService.conversationRepository.FindAll(executionContext)
	if findError != nil {
		return nil, findError
	}

	summaryDtos := make([]dto.ConversationSummaryDto, 0, len(conversations))
	for _, conversation := range conversations {
		summaryDtos = append(summaryDtos, domains.NewConversationDomain(conversation).ToSummaryDto())
	}

	return summaryDtos, nil
}

// GetConversation returns one whole conversation, every message included — including
// the ones too old for the assistant to still be shown.
func (assistantConversationService *AssistantConversationService) GetConversation(
	executionContext context.Context, id uint,
) (dto.ConversationDto, error) {
	conversation, findError := assistantConversationService.conversationRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.ConversationDto{}, findError
	}

	return domains.NewConversationDomain(conversation).ToDto(), nil
}

// recentMessagesOf is what the assistant is allowed to remember of the conversation
// this question belongs to. A question that names no conversation remembers nothing,
// because there is nothing yet to remember — and it must not be answered by inventing
// a conversation first, since an assistant that never answers must leave none behind.
func (assistantConversationService *AssistantConversationService) recentMessagesOf(
	executionContext context.Context, conversationId uint,
) ([]vo.AssistantMessageVo, error) {
	if conversationId == 0 {
		return make([]vo.AssistantMessageVo, 0), nil
	}

	conversation, findError := assistantConversationService.conversationRepository.FindOne(
		executionContext, conversationId)
	if findError != nil {
		return nil, findError
	}

	return domains.NewConversationDomain(conversation).RecentMessages(
		assistantConversationService.recentMessageLimit), nil
}

// writeAnswer drives the round trips until the assistant has written an answer.
//
// It ends because the queries are counted: once they are spent no further one runs,
// so the next round trip is the assistant's last chance to speak, and it is told so.
// An assistant that still says nothing then is treated as one that did not answer at
// all — recording a blank answer would put a question with nothing under it into the
// conversation for good.
func (assistantConversationService *AssistantConversationService) writeAnswer(
	executionContext context.Context, exchange domains.AssistantExchangeDomain,
) (domains.AssistantExchangeDomain, string, error) {
	for {
		reply, replyError := assistantConversationService.assistantProxy.Reply(
			executionContext, exchange.Request())
		if replyError != nil {
			return exchange, "", domains.AssistantUnavailable(replyError)
		}

		exchange = exchange.RecordUsage(reply.Usage)

		if reply.Answer != "" {
			return exchange, reply.Answer, nil
		}

		if len(reply.QueryCalls) == 0 || !exchange.AllowsQuery() {
			return exchange, "", domains.AssistantAnsweredNothing()
		}

		for _, call := range reply.QueryCalls {
			if !exchange.AllowsQuery() {
				break
			}

			outcome, rejected := assistantConversationService.runAssistantQuery(executionContext, call)
			exchange = exchange.RecordQuery(call, outcome, rejected)
		}
	}
}

// runAssistantQuery carries out what the assistant asked for and hands back what it
// should read, plus whether that reading is a refusal.
//
// This is where a refusal stops being a failure and becomes data, and it is named
// rather than inlined because that inversion is the whole point of it: whether the
// arguments broke a rule, the stretch held nothing, or the capability does not exist
// at all, the assistant is handed the reason and goes on writing. Ending the answer
// over any of them would throw away every lookup that had already succeeded.
func (assistantConversationService *AssistantConversationService) runAssistantQuery(
	executionContext context.Context, call vo.AssistantQueryCallVo,
) (string, bool) {
	for _, assistantQuery := range assistantConversationService.assistantQueries {
		if assistantQuery.Name() != call.Name {
			continue
		}

		outcome, runError := assistantQuery.Run(executionContext, call.Arguments)
		if runError != nil {
			return runError.Error(), true
		}

		return outcome, false
	}

	return "系統沒有「" + call.Name + "」這個能力。請改用已提供的能力，或告知使用者這件事辦不到。", true
}

// store puts the finished exchange away — as a new conversation when the question
// named none, as an addition when it did — and reports the answer with the
// conversation it now belongs to.
//
// Both ways of writing it are a single statement, which is what keeps "an assistant
// that never answered leaves nothing behind" free rather than a rollback somebody has
// to remember. The identifier is reported whether or not the caller supplied one, so
// that a question which started a conversation can be followed up without the caller
// going looking for where it landed.
func (assistantConversationService *AssistantConversationService) store(
	executionContext context.Context, conversationId uint, turn entities.AssistantTurn,
) (dto.AssistantAnswerDto, error) {
	storedConversation, storeError := entities.Conversation{}, error(nil)

	if conversationId == 0 {
		storedConversation, storeError = assistantConversationService.conversationRepository.Save(
			executionContext,
			entities.Conversation{
				LastActiveAt: turn.CreatedAt,
				Turns:        []entities.AssistantTurn{turn},
			})
	} else {
		storedConversation, storeError = assistantConversationService.conversationRepository.AppendTurn(
			executionContext, conversationId, turn)
	}

	if storeError != nil {
		return dto.AssistantAnswerDto{}, storeError
	}

	return dto.AssistantAnswerDto{
		ConversationID:      storedConversation.ID,
		Answer:              turn.Answer,
		QueryCount:          turn.QueryCount,
		StoppedAtQueryLimit: turn.StoppedAtQueryLimit,
		Usage:               turn.Usage,
	}, nil
}
