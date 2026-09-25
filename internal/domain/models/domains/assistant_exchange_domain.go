package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantExchangeDomain tracks one in-progress exchange (context, lookups so far, remaining queries, cost) so its cost rules are testable without an assistant.
// Recording returns a new value rather than mutating, since the same value is queried several times between round trips.
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

// Request builds the next round trip: recent messages with this question last, allowed queries, previous rounds and whether the query limit is reached.
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

// AllowedCalls returns the first calls that fit the remaining allowance, since the assistant may request several at once.
func (assistantExchangeDomain AssistantExchangeDomain) AllowedCalls(
	calls []vo.AssistantQueryCallVo,
) []vo.AssistantQueryCallVo {
	remaining := assistantExchangeDomain.queryRounds.Remaining()
	if len(calls) <= remaining {
		return calls
	}

	return calls[:remaining]
}

// RecordUsage adds every round trip's cost, including lookup-only trips, so long exchanges cannot bypass the allowance.
func (assistantExchangeDomain AssistantExchangeDomain) RecordUsage(usage int) AssistantExchangeDomain {
	recorded := assistantExchangeDomain
	recorded.usage = assistantExchangeDomain.usage + usage

	return recorded
}

// RecordRound keeps the round whole (narration plus all lookups) because that is how it is replayed to the assistant; see AssistantQueryRoundVo.
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

	// 序號從整次問答的第一次查詢連續數下去，而非每輪重新計。
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

// ToStartedTurn is stored before the assistant is asked, so the answer is visible while being written.
func (assistantExchangeDomain AssistantExchangeDomain) ToStartedTurn(
	at time.Time,
) entities.AssistantTurn {
	return entities.AssistantTurn{
		Ask:       assistantExchangeDomain.ask,
		Status:    string(vo.AssistantTurnRunning),
		CreatedAt: at.UTC(),
	}
}

// ToAnsweredTurn omits the question, which was fixed when the exchange began.
func (assistantExchangeDomain AssistantExchangeDomain) ToAnsweredTurn(
	turnID uint, answer string,
) entities.AssistantTurn {
	return entities.AssistantTurn{
		ID:                  turnID,
		Answer:              answer,
		Status:              string(vo.AssistantTurnAnswered),
		Usage:               assistantExchangeDomain.usage,
		QueryCount:          assistantExchangeDomain.queryRounds.Used(),
		StoppedAtQueryLimit: assistantExchangeDomain.queryRounds.ReachedLimit(),
		Queries:             assistantExchangeDomain.records,
	}
}

// ToFailedTurn deliberately records zero usage so failed attempts never consume the person's daily allowance, and drops the lookups since there is no answer to explain.
func (assistantExchangeDomain AssistantExchangeDomain) ToFailedTurn(
	turnID uint, reason string,
) entities.AssistantTurn {
	return entities.AssistantTurn{
		ID:            turnID,
		Status:        string(vo.AssistantTurnFailed),
		FailureReason: reason,
		QueryCount:    assistantExchangeDomain.queryRounds.Used(),
	}
}
