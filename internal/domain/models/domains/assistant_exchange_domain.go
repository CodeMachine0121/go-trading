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
	rounds            []vo.AssistantQueryRoundVo
	records           []entities.AssistantQueryRecord
	queryRounds       AssistantQueryRoundsDomain
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
		rounds:            make([]vo.AssistantQueryRoundVo, 0),
		records:           make([]entities.AssistantQueryRecord, 0),
		queryRounds:       NewAssistantQueryRoundsDomain(queryLimit),
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
		Rounds:            assistantExchangeDomain.rounds,
		QueryLimitReached: assistantExchangeDomain.queryRounds.ReachedLimit(),
		AnswerLengthLimit: assistantExchangeDomain.answerLengthLimit,
	}
}

// AllowedCalls is which of the lookups the assistant just asked for may actually
// run: the first few, up to what this exchange has left.
//
// It answers with a slice rather than a yes or no because the assistant asks for
// several at once. Asked "may I run these five?" with two left, the honest answer
// is "the first two" — refusing all five throws away work the assistant is entitled
// to, and running all five is how a ceiling stops being a ceiling.
func (assistantExchangeDomain AssistantExchangeDomain) AllowedCalls(
	calls []vo.AssistantQueryCallVo,
) []vo.AssistantQueryCallVo {
	remaining := assistantExchangeDomain.queryRounds.Remaining()
	if len(calls) <= remaining {
		return calls
	}

	return calls[:remaining]
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

// RecordRound notes one round trip that looked things up: what the assistant said
// on the way, and every lookup of that round with what came back.
//
// The round is kept whole rather than flattened into single lookups, because that
// is how it goes back to the assistant — its narration with its requests, and all
// of that round's results in one reply. See AssistantQueryRoundVo for what breaks
// when either half is split.
func (assistantExchangeDomain AssistantExchangeDomain) RecordRound(
	narration string, exchanges []vo.AssistantQueryExchangeVo,
) AssistantExchangeDomain {
	if len(exchanges) == 0 {
		return assistantExchangeDomain
	}

	recorded := assistantExchangeDomain
	recorded.queryRounds = assistantExchangeDomain.queryRounds.Record(len(exchanges))

	recorded.rounds = append(
		append(make([]vo.AssistantQueryRoundVo, 0, len(assistantExchangeDomain.rounds)+1),
			assistantExchangeDomain.rounds...),
		vo.AssistantQueryRoundVo{Narration: narration, Exchanges: exchanges})

	records := make([]entities.AssistantQueryRecord, 0,
		len(assistantExchangeDomain.records)+len(exchanges))
	records = append(records, assistantExchangeDomain.records...)

	// 序號從整次問答的第一次查詢數下去，不是從這一輪的第一次——
	// 讀紀錄的人問的是「這次回答查了什麼」，那是一條連續的鏈。
	sequence := len(assistantExchangeDomain.records)
	for _, exchange := range exchanges {
		sequence++
		records = append(records, entities.AssistantQueryRecord{
			Sequence:  sequence,
			QueryName: exchange.Call.Name,
			Arguments: exchange.Call.Arguments,
			Outcome:   exchange.Outcome,
			Rejected:  exchange.Rejected,
		})
	}

	recorded.records = records

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
		QueryCount:          assistantExchangeDomain.queryRounds.Used(),
		StoppedAtQueryLimit: assistantExchangeDomain.queryRounds.ReachedLimit(),
		CreatedAt:           at.UTC(),
		Queries:             assistantExchangeDomain.records,
	}
}
