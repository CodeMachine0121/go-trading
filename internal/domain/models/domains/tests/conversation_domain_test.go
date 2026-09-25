package domains_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conversationOf numbers its exchanges so a test can say which ones survive the trim.
func conversationOf(exchangeCount int) entities.Conversation {
	turns := make([]entities.AssistantTurn, 0, exchangeCount)
	for exchangeNumber := 1; exchangeNumber <= exchangeCount; exchangeNumber++ {
		turns = append(turns, entities.AssistantTurn{
			Ask:       fmt.Sprintf("問題 %d", exchangeNumber),
			Answer:    fmt.Sprintf("回答 %d", exchangeNumber),
			CreatedAt: time.Date(2026, 9, 4, 10, exchangeNumber, 0, 0, time.UTC),
		})
	}

	return entities.Conversation{
		ID:           7,
		LastActiveAt: time.Date(2026, 9, 4, 10, exchangeCount, 0, 0, time.UTC),
		Turns:        turns,
	}
}

func TestConversationDomainShowsTheAssistantAtMostTheRecentLimit(t *testing.T) {
	testCases := []struct {
		name                 string
		exchangeCount        int
		limit                int
		expectedMessageCount int
		expectedFirstContent string
		expectedLastContent  string
	}{
		{
			name: "well within the limit shows everything", exchangeCount: 3, limit: 20,
			expectedMessageCount: 6, expectedFirstContent: "問題 1", expectedLastContent: "回答 3",
		},
		{
			name: "exactly the limit shows everything", exchangeCount: 10, limit: 20,
			expectedMessageCount: 20, expectedFirstContent: "問題 1", expectedLastContent: "回答 10",
		},
		{
			// 13 exchanges are 26 messages, so the six oldest drop and the fourth exchange's question comes first.
			name: "past the limit shows only the most recent", exchangeCount: 13, limit: 20,
			expectedMessageCount: 20, expectedFirstContent: "問題 4", expectedLastContent: "回答 13",
		},
		{
			name: "a limit of one shows only the last thing said", exchangeCount: 3, limit: 1,
			expectedMessageCount: 1, expectedFirstContent: "回答 3", expectedLastContent: "回答 3",
		},
		{
			name: "a conversation with nothing in it shows nothing", exchangeCount: 0, limit: 20,
			expectedMessageCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conversationDomain := domains.NewConversationDomain(conversationOf(testCase.exchangeCount))

			recentMessages := conversationDomain.RecentMessages(testCase.limit)

			require.Len(t, recentMessages, testCase.expectedMessageCount)
			if testCase.expectedMessageCount == 0 {
				return
			}
			assert.Equal(t, testCase.expectedFirstContent, recentMessages[0].Content)
			assert.Equal(t, testCase.expectedLastContent, recentMessages[len(recentMessages)-1].Content)
		})
	}
}

func TestConversationDomainShowsTheAssistantOnlyQuestionsAndAnswers(t *testing.T) {
	// Earlier lookups are not resent; they were only needed while that answer was written.
	conversation := conversationOf(1)
	conversation.Turns[0].Queries = []entities.AssistantQueryRecord{
		{Sequence: 1, QueryName: "get_k_candle_series", Outcome: "一大包 K 線"},
	}

	recentMessages := domains.NewConversationDomain(conversation).RecentMessages(20)

	require.Len(t, recentMessages, 2)
	assert.Equal(t, vo.AssistantMessageRoleAsk, recentMessages[0].Role)
	assert.Equal(t, vo.AssistantMessageRoleAnswer, recentMessages[1].Role)
	for _, message := range recentMessages {
		assert.NotContains(t, message.Content, "一大包 K 線")
	}
}

func TestConversationDomainToDtoKeepsEveryMessageEverSaid(t *testing.T) {
	// The assistant's memory is trimmed but the reader's record is not.
	conversationDto := domains.NewConversationDomain(conversationOf(13)).ToDto()

	assert.Equal(t, uint(7), conversationDto.ID)
	require.Len(t, conversationDto.Messages, 26)
	assert.Equal(t, "問題 1", conversationDto.Messages[0].Content)
	assert.Equal(t, "ask", conversationDto.Messages[0].Role)
	assert.Equal(t, "answer", conversationDto.Messages[1].Role)
	assert.Equal(t, "回答 13", conversationDto.Messages[25].Content)
}

func TestConversationDomainToDtoHandsOutTimesInUniversalTime(t *testing.T) {
	conversation := conversationOf(1)
	elsewhere := time.FixedZone("UTC+8", 8*60*60)
	conversation.LastActiveAt = conversation.LastActiveAt.In(elsewhere)
	conversation.Turns[0].CreatedAt = conversation.Turns[0].CreatedAt.In(elsewhere)

	conversationDto := domains.NewConversationDomain(conversation).ToDto()

	assert.Equal(t, time.UTC, conversationDto.LastActiveAt.Location())
	assert.Equal(t, time.UTC, conversationDto.Messages[0].CreatedAt.Location())
}

func TestConversationDomainToSummaryDtoCountsWhatIsInIt(t *testing.T) {
	summaryDto := domains.NewConversationDomain(conversationOf(3)).ToSummaryDto()

	assert.Equal(t, uint(7), summaryDto.ID)
	assert.Equal(t, 6, summaryDto.MessageCount)
	assert.Equal(t, time.Date(2026, 9, 4, 10, 3, 0, 0, time.UTC), summaryDto.LastActiveAt)
}

func TestConversationDomainToSummaryDtoCountsNothingAsNothing(t *testing.T) {
	summaryDto := domains.NewConversationDomain(conversationOf(0)).ToSummaryDto()

	assert.Equal(t, 0, summaryDto.MessageCount)
}

// conversationEndingWith precedes the case's turn with one answered exchange, so tests also check finished ones are untouched.
func conversationEndingWith(turn entities.AssistantTurn) entities.Conversation {
	conversation := conversationOf(1)
	conversation.Turns[0].Status = string(vo.AssistantTurnAnswered)
	conversation.Turns = append(conversation.Turns, turn)

	return conversation
}

func TestConversationDomainToDtoSaysWhatStateEachExchangeIsIn(t *testing.T) {
	// Unfinished exchanges must be shown with their state, or the reader sees a blank and asks again.
	testCases := []struct {
		name                  string
		turn                  entities.AssistantTurn
		expectedMessageCount  int
		expectedLastRole      string
		expectedLastStatus    string
		expectedFailureReason string
	}{
		{
			name: "one still being written shows the question and no reply yet",
			turn: entities.AssistantTurn{
				Ask: "還在跑的那句", Status: string(vo.AssistantTurnRunning),
				CreatedAt: time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
			},
			expectedMessageCount: 3,
			expectedLastRole:     "ask",
			expectedLastStatus:   "running",
		},
		{
			name: "one that failed says why",
			turn: entities.AssistantTurn{
				Ask: "壞掉的那句", Status: string(vo.AssistantTurnFailed),
				FailureReason: "助手目前沒有回應，請稍後再試",
				CreatedAt:     time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
			},
			expectedMessageCount:  3,
			expectedLastRole:      "ask",
			expectedLastStatus:    "failed",
			expectedFailureReason: "助手目前沒有回應，請稍後再試",
		},
		{
			name: "one that was answered has both halves",
			turn: entities.AssistantTurn{
				Ask: "答完的那句", Answer: "答案", Status: string(vo.AssistantTurnAnswered),
				CreatedAt: time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
			},
			expectedMessageCount: 4,
			expectedLastRole:     "answer",
			expectedLastStatus:   "answered",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conversationDto := domains.NewConversationDomain(
				conversationEndingWith(testCase.turn)).ToDto()

			require.Len(t, conversationDto.Messages, testCase.expectedMessageCount)

			lastMessage := conversationDto.Messages[len(conversationDto.Messages)-1]
			assert.Equal(t, testCase.expectedLastRole, lastMessage.Role)
			assert.Equal(t, testCase.expectedLastStatus, lastMessage.Status)
			assert.Equal(t, testCase.expectedFailureReason, lastMessage.FailureReason)

			assert.Equal(t, "問題 1", conversationDto.Messages[0].Content)
			assert.Equal(t, "answered", conversationDto.Messages[0].Status)
		})
	}
}

func TestConversationDomainDoesNotShowTheAssistantAQuestionThatWasNeverAnswered(t *testing.T) {
	// An unanswered question would read to the assistant as one it declined.
	conversation := conversationEndingWith(entities.AssistantTurn{
		Ask: "壞掉的那句", Status: string(vo.AssistantTurnFailed),
		FailureReason: "助手沒有回應",
		CreatedAt:     time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
	})

	recentMessages := domains.NewConversationDomain(conversation).RecentMessages(20)

	require.Len(t, recentMessages, 2)
	assert.Equal(t, "問題 1", recentMessages[0].Content)
	assert.Equal(t, "回答 1", recentMessages[1].Content)
}

func TestConversationDomainKnowsWhetherAnAnswerIsStillBeingWritten(t *testing.T) {
	// Only an in-flight exchange blocks the next question; a failed one has nothing still being written.
	testCases := []struct {
		name             string
		status           string
		expectedInFlight bool
	}{
		{name: "one still being written", status: string(vo.AssistantTurnRunning), expectedInFlight: true},
		{name: "one that was answered", status: string(vo.AssistantTurnAnswered), expectedInFlight: false},
		{name: "one that failed", status: string(vo.AssistantTurnFailed), expectedInFlight: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conversation := conversationEndingWith(entities.AssistantTurn{
				Ask: "最後那句", Status: testCase.status,
				CreatedAt: time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
			})

			assert.Equal(t, testCase.expectedInFlight,
				domains.NewConversationDomain(conversation).HasAnswerInFlight())
		})
	}
}

func TestConversationDomainReadsAnExchangeStoredBeforeStatesExistedAsAnswered(t *testing.T) {
	// Legacy rows with no status and a full answer read as answered.
	conversationDto := domains.NewConversationDomain(conversationOf(1)).ToDto()

	require.Len(t, conversationDto.Messages, 2)
	assert.Equal(t, "answered", conversationDto.Messages[0].Status)
	assert.Equal(t, "回答 1", conversationDto.Messages[1].Content)
}

func TestConversationDomainToDtoCarriesWhatAnAnswerCost(t *testing.T) {
	// This record is the only place the query counts survive.
	conversation := conversationEndingWith(entities.AssistantTurn{
		Ask: "查到底", Answer: "只查到這些", Status: string(vo.AssistantTurnAnswered),
		QueryCount: 40, StoppedAtQueryLimit: true, Usage: 12000,
		CreatedAt: time.Date(2026, 9, 4, 10, 2, 0, 0, time.UTC),
	})

	conversationDto := domains.NewConversationDomain(conversation).ToDto()

	require.Len(t, conversationDto.Messages, 4)
	answerMessage := conversationDto.Messages[3]
	assert.Equal(t, "answer", answerMessage.Role)
	assert.Equal(t, 40, answerMessage.QueryCount)
	assert.True(t, answerMessage.StoppedAtQueryLimit)
	assert.Equal(t, 12000, answerMessage.Usage)

	// Cost belongs to the answer, not the question.
	assert.Equal(t, 0, conversationDto.Messages[2].QueryCount)
	assert.Equal(t, 0, conversationDto.Messages[2].Usage)
}

func TestConversationDomainToDtoKeepsFinishedAndUnfinishedExchangesInOrder(t *testing.T) {
	// The unfinished exchange is read back in order as the last one, not hoisted or dropped.
	conversation := conversationOf(3)
	for index := range conversation.Turns {
		conversation.Turns[index].Status = string(vo.AssistantTurnAnswered)
	}
	conversation.Turns = append(conversation.Turns, entities.AssistantTurn{
		Ask: "還在跑的", Status: string(vo.AssistantTurnRunning),
		CreatedAt: time.Date(2026, 9, 4, 10, 4, 0, 0, time.UTC),
	})

	conversationDto := domains.NewConversationDomain(conversation).ToDto()

	// Three finished exchanges of two messages, plus the unfinished question alone.
	require.Len(t, conversationDto.Messages, 7)
	assert.Equal(t, []string{
		"問題 1", "回答 1", "問題 2", "回答 2", "問題 3", "回答 3", "還在跑的",
	}, contentsOf(conversationDto.Messages))
	assert.Equal(t, "running", conversationDto.Messages[6].Status)
}

func contentsOf(messages []dto.ConversationMessageDto) []string {
	contents := make([]string, 0, len(messages))
	for _, message := range messages {
		contents = append(contents, message.Content)
	}

	return contents
}
