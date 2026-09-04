package domains_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conversationOf is a conversation holding this many exchanges, numbered so that a
// test can say which one it expected to survive the trim.
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
			// Thirteen exchanges are twenty-six messages, so the six oldest fall
			// away and the fourth exchange's question is the first thing still shown.
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
	// What an earlier exchange looked up was worth its cost once, when the answer
	// that needed it was being written. Sending it again buys nothing.
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
	// What the assistant may remember and what a person may read are different
	// questions. Trimming both by one number would erase the record as well.
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
