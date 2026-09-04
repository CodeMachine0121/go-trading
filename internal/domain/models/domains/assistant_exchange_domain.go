package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantExchangeDomain is one exchange while it is still being written: what the
// assistant may see, what it has looked at so far, how many queries it has left, and
// what the round trips have cost.
//
// It exists so that the bookkeeping of an exchange lives next to the exchange rather
// than as a handful of variables in whoever is driving the round trips. Every rule
// about what an exchange may cost is asked of this one value, which is also what
// makes those rules testable without an assistant on the other end.
//
// Recording hands back a new value rather than changing this one, the same way the
// query count does: within one exchange the same value is asked several questions
// between round trips, and one that changed underneath the asker would be a value
// nobody could reason about.
type AssistantExchangeDomain struct {
	ask               string
	recentMessages    []vo.AssistantMessageVo
	declarations      []vo.AssistantQueryDeclarationVo
	exchanges         []vo.AssistantQueryExchangeVo
	records           []entities.AssistantQueryRecord
	rounds            AssistantQueryRoundsDomain
	usage             int
	answerLengthLimit int
}

// NewAssistantExchangeDomain starts an exchange: the question being answered, what
// the assistant is allowed to remember, what it is allowed to do, and the ceilings
// this exchange obeys.
func NewAssistantExchangeDomain(
	ask string,
	recentMessages []vo.AssistantMessageVo,
	declarations []vo.AssistantQueryDeclarationVo,
	queryLimit int,
	answerLengthLimit int,
) AssistantExchangeDomain {
	return AssistantExchangeDomain{
		ask:               ask,
		recentMessages:    recentMessages,
		declarations:      declarations,
		exchanges:         make([]vo.AssistantQueryExchangeVo, 0),
		records:           make([]entities.AssistantQueryRecord, 0),
		rounds:            NewAssistantQueryRoundsDomain(queryLimit),
		usage:             0,
		answerLengthLimit: answerLengthLimit,
	}
}

// Request is everything the next round trip is allowed to show the assistant: the
// recent messages with this question last, what it may do, what it has looked at so
// far, and whether it is out of queries and must answer now.
func (assistantExchangeDomain AssistantExchangeDomain) Request() vo.AssistantTurnRequestVo {
	messages := make([]vo.AssistantMessageVo, 0, len(assistantExchangeDomain.recentMessages)+1)
	messages = append(messages, assistantExchangeDomain.recentMessages...)
	messages = append(messages, vo.AssistantMessageVo{
		Role:    vo.AssistantMessageRoleAsk,
		Content: assistantExchangeDomain.ask,
	})

	return vo.AssistantTurnRequestVo{
		Messages:          messages,
		Declarations:      assistantExchangeDomain.declarations,
		Exchanges:         assistantExchangeDomain.exchanges,
		QueryLimitReached: assistantExchangeDomain.rounds.ReachedLimit(),
		AnswerLengthLimit: assistantExchangeDomain.answerLengthLimit,
	}
}

// AllowsQuery says whether one more assistant query may run within this exchange.
func (assistantExchangeDomain AssistantExchangeDomain) AllowsQuery() bool {
	return assistantExchangeDomain.rounds.Allows()
}

// RecordUsage adds what a round trip cost. Every round trip is added, not just the
// one that answered: a trip that only asked for a lookup was still paid for, and an
// allowance that could not see those trips would be an allowance a long exchange
// walks straight through.
func (assistantExchangeDomain AssistantExchangeDomain) RecordUsage(usage int) AssistantExchangeDomain {
	recorded := assistantExchangeDomain
	recorded.usage = assistantExchangeDomain.usage + usage

	return recorded
}

// RecordQuery notes one assistant query and what came back from it, both for the
// assistant to read on the next round trip and for the record that outlives the
// exchange.
func (assistantExchangeDomain AssistantExchangeDomain) RecordQuery(
	call vo.AssistantQueryCallVo, outcome string, rejected bool,
) AssistantExchangeDomain {
	recorded := assistantExchangeDomain
	recorded.rounds = assistantExchangeDomain.rounds.Record()

	recorded.exchanges = append(
		append(make([]vo.AssistantQueryExchangeVo, 0, len(assistantExchangeDomain.exchanges)+1),
			assistantExchangeDomain.exchanges...),
		vo.AssistantQueryExchangeVo{Call: call, Outcome: outcome, Rejected: rejected})

	recorded.records = append(
		append(make([]entities.AssistantQueryRecord, 0, len(assistantExchangeDomain.records)+1),
			assistantExchangeDomain.records...),
		entities.AssistantQueryRecord{
			Sequence:  recorded.rounds.Used(),
			QueryName: call.Name,
			Arguments: call.Arguments,
			Outcome:   outcome,
			Rejected:  rejected,
		})

	return recorded
}

// ToTurn is this exchange as it will be stored, once the assistant has answered.
func (assistantExchangeDomain AssistantExchangeDomain) ToTurn(
	answer string, at time.Time,
) entities.AssistantTurn {
	return entities.AssistantTurn{
		Ask:                 assistantExchangeDomain.ask,
		Answer:              answer,
		Usage:               assistantExchangeDomain.usage,
		QueryCount:          assistantExchangeDomain.rounds.Used(),
		StoppedAtQueryLimit: assistantExchangeDomain.rounds.ReachedLimit(),
		CreatedAt:           at.UTC(),
		Queries:             assistantExchangeDomain.records,
	}
}
