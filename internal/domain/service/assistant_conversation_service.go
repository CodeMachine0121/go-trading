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
// Asking is one call in and a place to look out. Whoever calls it does not check the
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

// Ask takes one question, reserves the place its answer will go, and sets the answer
// being written somewhere the caller is not waiting.
//
// **It does not return an answer, and that is the change this design turns on.** The
// assistant may go round dozens of times — building a set of rules, replaying it,
// adjusting it, replaying again — and holding the asker on a connection for that is
// what makes a refresh lose everything and a closed tab kill work already done. What
// comes back instead is where the answer will appear.
//
// The order of the first three checks is the order the refusals cost least in. A
// question with nothing in it is refused before the day's usage is read, and the
// day's usage before the conversation is fetched, so that the cheapest refusal never
// pays for the more expensive one's lookup. It also keeps the refusals honest: a
// blank question sent to a conversation that does not exist is answered as a blank
// question, which is the thing the sender can actually fix.
//
// **Nothing is stored until all four checks pass.** That is what keeps "a question
// that was never accepted leaves nothing behind" true — the guarantee that used to
// cover a failed answer as well, and no longer does. A failed answer now leaves a row
// saying so, deliberately: somebody who was not watching has to be able to tell it
// apart from one still running and from one they never sent.
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

// ListConversations returns this person's conversations, the most recently active
// first. Holding none is an answer rather than a failure.
//
// Whose they are is asked of the store rather than sorted out here: nothing this
// method could forget to do can put somebody else's conversation in the list.
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

// GetConversation returns one whole conversation of this person's own, every message
// included — including the ones too old for the assistant to still be shown.
//
// Somebody else's is answered as one that is not there. A transcript is not only
// what was said: the assistant acts as whoever asked it, so an exchange can hold
// that person's own algorithms in full.
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

// recentMessagesOf is what the assistant is allowed to remember of the conversation
// this question belongs to. A question that names no conversation remembers nothing,
// because there is nothing yet to remember — and it must not be answered by inventing
// a conversation first, since a question that was never accepted must leave none
// behind.
//
// This is also where a question aimed at somebody else's conversation is turned away,
// and where one arriving while the previous answer is still being written is. Both
// belong here for the same reason: this is the only read of that conversation before
// the exchange is appended to it, so passing here is what makes the append safe,
// rather than a second check that could disagree with this one.
//
// A conversation that has not been started yet can have nothing in flight, so the
// question does not arise on that path.
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

	// Asked after ownership, so that somebody probing a stranger's conversation is
	// told it does not exist rather than told it is busy.
	if conversationDomain.HasAnswerInFlight() {
		return nil, domains.AssistantAnswerInProgress()
	}

	return conversationDomain.RecentMessages(
		assistantConversationService.recentMessageLimit), nil
}

// writeAnswer drives the round trips until the assistant has written an answer.
//
// **A reply that asks for lookups is not an answer, even when it also says
// something.** The assistant routinely does both in one breath — "let me check the
// existing scripts first" alongside the lookup it wants — and reading that sentence
// as the answer ends the turn before a single lookup has run. What the reader then
// gets is a promise, and the only way forward is to ask "well?", which is not a
// conversation. So the lookups are checked first and the sentence travels with them
// as narration.
//
// It ends because the queries are counted: once they are spent no further one runs,
// so the next round trip is the assistant's last chance to speak, and it is told so.
// An assistant that still says nothing then is treated as one that did not answer at
// all — recording a blank answer would put a question with nothing under it into the
// conversation for good.
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

		// 先問「有沒有要查、還准不准查」，再問「有沒有說話」。順序反過來就是上面那個
		// 「我先看一下…」然後結束的問題。
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

// runAssistantQueries carries out this round's lookups and hands back what the
// assistant should read for each.
//
// They run in the order they were asked for, and every one of them produces a
// result — including the refused ones. A request left without a result is the one
// shape the assistant's own interface refuses outright.
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

// runAssistantQuery carries out what the assistant asked for and hands back what it
// should read, plus whether that reading is a refusal.
//
// This is where a refusal stops being a failure and becomes data, and it is named
// rather than inlined because that inversion is the whole point of it: whether the
// arguments broke a rule, the stretch held nothing, or the capability does not exist
// at all, the assistant is handed the reason and goes on writing. Ending the answer
// over any of them would throw away every lookup that had already succeeded.
func (assistantConversationService *AssistantConversationService) runAssistantQuery(
	executionContext context.Context, viewerID uint, call vo.AssistantQueryCallVo,
) (string, bool) {
	for _, assistantQuery := range assistantConversationService.assistantQueries {
		if assistantQuery.Name() != call.Name {
			continue
		}

		outcome, runError := assistantQuery.Run(executionContext, viewerID, call.Arguments)
		if runError != nil {
			return runError.Error(), true
		}

		return outcome, false
	}

	return "系統沒有「" + call.Name + "」這個能力。請改用已提供的能力，或告知使用者這件事辦不到。", true
}

// start reserves the place an answer will go — as a new conversation when the
// question named none, as an addition when it did — and names both the conversation
// it landed in and the exchange itself.
//
// Both ways of writing it are a single statement, so a question never lands half
// stored: a conversation with no exchange under it would show up in somebody's list
// as an empty thread they never started.
//
// The exchange is named by what the store gave it rather than by looking for the
// newest one afterwards. Two questions arriving at once would both find the same
// newest exchange, and one answer would be written over the other.
//
// This runs on the asker's own context rather than a detached one, unlike the writing
// that follows. Reserving the place is the part they are waiting on, so a caller who
// gives up before it lands should take it with them.
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

// FailInterruptedAnswers marks every answer left mid-write by the last shutdown as
// failed, and says how many there were.
//
// It belongs on this service rather than in a job of its own because the rule it
// enforces is this service's: an answer being written lives in this process and
// nowhere else, so a row still saying running is a claim about a process that no
// longer exists. Left alone it is a wait nobody can end, on a conversation nobody can
// add to.
//
// It is a public use case and calls none of the others, as every method here does.
func (assistantConversationService *AssistantConversationService) FailInterruptedAnswers(
	executionContext context.Context,
) (int, error) {
	return assistantConversationService.conversationRepository.FailAllRunningTurns(
		executionContext, domains.AssistantAnswerInterruptedByRestart())
}
