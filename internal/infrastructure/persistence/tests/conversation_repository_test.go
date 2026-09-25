package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func turnAt(moment time.Time, ask string, usage int) entities.AssistantTurn {
	return entities.AssistantTurn{
		Ask:       ask,
		Answer:    "答:" + ask,
		Usage:     usage,
		CreatedAt: moment,
	}
}

func momentAt(hour int, minute int) time.Time {
	return time.Date(2026, 9, 4, hour, minute, 0, 0, time.UTC)
}

func TestConversationRepositorySaveStoresTheConversationWithItsFirstExchange(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})

	require.NoError(t, saveError)
	assert.Positive(t, savedConversation.ID)
	assert.False(t, savedConversation.CreatedAt.IsZero())
	require.Len(t, savedConversation.Turns, 1)
	assert.Positive(t, savedConversation.Turns[0].ID)
}

func TestConversationRepositoryStoresWhatEachExchangeLookedAt(t *testing.T) {
	// Lookups are never resent to the assistant, so this record is their only copy.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	turn := turnAt(momentAt(10, 0), "問 1", 100)
	turn.QueryCount = 2
	turn.StoppedAtQueryLimit = true
	turn.Queries = []entities.AssistantQueryRecord{
		{Sequence: 2, QueryName: "get_k_candles", Arguments: `{}`, Outcome: "被拒", Rejected: true},
		{Sequence: 1, QueryName: "list_trading_symbols", Arguments: `{}`, Outcome: `{"symbols":[]}`},
	}

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turn},
	})
	require.NoError(t, saveError)

	readBackConversation, findError := conversationRepository.FindOne(t.Context(), savedConversation.ID)

	require.NoError(t, findError)
	require.Len(t, readBackConversation.Turns, 1)
	assert.Equal(t, 2, readBackConversation.Turns[0].QueryCount)
	assert.True(t, readBackConversation.Turns[0].StoppedAtQueryLimit)
	require.Len(t, readBackConversation.Turns[0].Queries, 2)
	assert.Equal(t, 1, readBackConversation.Turns[0].Queries[0].Sequence)
	assert.Equal(t, "list_trading_symbols", readBackConversation.Turns[0].Queries[0].QueryName)
	assert.Equal(t, 2, readBackConversation.Turns[0].Queries[1].Sequence)
	assert.True(t, readBackConversation.Turns[0].Queries[1].Rejected)
}

func TestConversationRepositoryAppendTurnAddsToWhatIsAlreadyThere(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})
	require.NoError(t, saveError)

	appendedTurn, appendError := conversationRepository.AppendTurn(
		t.Context(), savedConversation.ID, turnAt(momentAt(11, 0), "問 2", 200))

	require.NoError(t, appendError)
	// The appended turn identifies its own row, so a later answer lands on it even if another question arrived at the same moment.
	assert.Equal(t, "問 2", appendedTurn.Ask)
	assert.Positive(t, appendedTurn.ID)
	assert.NotEqual(t, savedConversation.Turns[0].ID, appendedTurn.ID)

	readBackConversation, findError := conversationRepository.FindOne(
		t.Context(), savedConversation.ID)
	require.NoError(t, findError)
	require.Len(t, readBackConversation.Turns, 2)
	assert.Equal(t, "問 1", readBackConversation.Turns[0].Ask)
	assert.Equal(t, "問 2", readBackConversation.Turns[1].Ask)
	assert.Equal(t, momentAt(11, 0), readBackConversation.LastActiveAt.UTC())
}

func TestConversationRepositoryAppendTurnReportsAConversationThatIsNotThere(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	_, appendError := conversationRepository.AppendTurn(
		t.Context(), 999, turnAt(momentAt(11, 0), "問 1", 100))

	require.ErrorIs(t, appendError, domains.ErrConversationNotFound)
}

func TestConversationRepositoryFindOneReportsOneThatIsNotThere(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	_, findError := conversationRepository.FindOne(t.Context(), 999)

	require.ErrorIs(t, findError, domains.ErrConversationNotFound)
}

func TestConversationRepositoryFindAllOwnedByPutsTheMostRecentlyActiveFirst(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	_, oldestError := conversationRepository.Save(t.Context(), entities.Conversation{
		OwnerID:      7,
		LastActiveAt: momentAt(9, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(9, 0), "舊的", 100)},
	})
	require.NoError(t, oldestError)
	_, newestError := conversationRepository.Save(t.Context(), entities.Conversation{
		OwnerID:      7,
		LastActiveAt: momentAt(12, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(12, 0), "新的", 100)},
	})
	require.NoError(t, newestError)

	conversations, findError := conversationRepository.FindAllOwnedBy(t.Context(), 7)

	require.NoError(t, findError)
	require.Len(t, conversations, 2)
	assert.Equal(t, momentAt(12, 0), conversations[0].LastActiveAt.UTC())
	// Turns are preloaded because the list shows each conversation's message count.
	require.Len(t, conversations[0].Turns, 1)
	assert.Equal(t, "新的", conversations[0].Turns[0].Ask)
	assert.Equal(t, momentAt(9, 0), conversations[1].LastActiveAt.UTC())
}

func TestConversationRepositoryFindAllOwnedByAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	conversations, findError := conversationRepository.FindAllOwnedBy(t.Context(), 7)

	require.NoError(t, findError)
	assert.Empty(t, conversations)
}

func TestConversationRepositoryFindAllOwnedByLeavesOutSomebodyElses(t *testing.T) {
	// Transcripts can hold the owner's private algorithms, so ownership must be a read condition.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	_, mineError := conversationRepository.Save(t.Context(), entities.Conversation{
		OwnerID:      7,
		LastActiveAt: momentAt(9, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(9, 0), "我問的", 100)},
	})
	require.NoError(t, mineError)
	_, theirsError := conversationRepository.Save(t.Context(), entities.Conversation{
		OwnerID:      8,
		LastActiveAt: momentAt(12, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(12, 0), "別人問的", 100)},
	})
	require.NoError(t, theirsError)

	conversations, findError := conversationRepository.FindAllOwnedBy(t.Context(), 7)

	require.NoError(t, findError)
	require.Len(t, conversations, 1)
	assert.Equal(t, "我問的", conversations[0].Turns[0].Ask)
}

func TestConversationRepositorySumUsageBetweenTotalsTheStretchAcrossEveryConversation(t *testing.T) {
	// The allowance is per day across all conversations.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	firstConversation, firstError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})
	require.NoError(t, firstError)
	_, appendError := conversationRepository.AppendTurn(
		t.Context(), firstConversation.ID, turnAt(momentAt(11, 0), "問 2", 250))
	require.NoError(t, appendError)
	_, secondError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(12, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(12, 0), "問 3", 400)},
	})
	require.NoError(t, secondError)

	total, sumError := conversationRepository.SumUsageBetween(
		t.Context(), momentAt(0, 0), momentAt(23, 59))

	require.NoError(t, sumError)
	assert.Equal(t, 750, total)
}

func TestConversationRepositorySumUsageBetweenIncludesTheStartAndExcludesTheEnd(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	_, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})
	require.NoError(t, saveError)

	testCases := []struct {
		name          string
		from          time.Time
		to            time.Time
		expectedTotal int
	}{
		{
			name: "the exchange sits exactly on the start", from: momentAt(10, 0), to: momentAt(11, 0),
			expectedTotal: 100,
		},
		{
			name: "the exchange sits exactly on the end", from: momentAt(9, 0), to: momentAt(10, 0),
			expectedTotal: 0,
		},
		{
			name: "a stretch that holds nothing totals zero", from: momentAt(20, 0), to: momentAt(21, 0),
			expectedTotal: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			total, sumError := conversationRepository.SumUsageBetween(t.Context(), testCase.from, testCase.to)

			require.NoError(t, sumError)
			assert.Equal(t, testCase.expectedTotal, total)
		})
	}
}

func TestConversationRepositoryReportsStorageThatIsNotThere(t *testing.T) {
	// A storage failure must not be reported as a missing conversation.
	database := newTestDatabase(t)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	conversationRepository := persistence.NewConversationRepository(database)

	_, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})
	assert.ErrorContains(t, saveError, "save conversation")

	_, appendError := conversationRepository.AppendTurn(
		t.Context(), 1, turnAt(momentAt(10, 0), "問 1", 100))
	assert.ErrorContains(t, appendError, "append conversation turn")
	assert.NotErrorIs(t, appendError, domains.ErrConversationNotFound)

	_, findOneError := conversationRepository.FindOne(t.Context(), 1)
	assert.ErrorContains(t, findOneError, "find conversation")
	assert.NotErrorIs(t, findOneError, domains.ErrConversationNotFound)

	_, findAllError := conversationRepository.FindAllOwnedBy(t.Context(), 7)
	assert.ErrorContains(t, findAllError, "find conversations")

	_, sumError := conversationRepository.SumUsageBetween(t.Context(), momentAt(0, 0), momentAt(23, 59))
	assert.ErrorContains(t, sumError, "sum assistant usage")
}

func TestConversationRepositoryReportsAnExchangeTheStoreWillNotAccept(t *testing.T) {
	// PostgreSQL refuses NUL in text; the refusal must surface as a storage failure, not as not found.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "問 1", 100)},
	})
	require.NoError(t, saveError)

	_, appendError := conversationRepository.AppendTurn(
		t.Context(), savedConversation.ID, turnAt(momentAt(11, 0), "問\x00題", 100))

	require.Error(t, appendError)
	assert.NotErrorIs(t, appendError, domains.ErrConversationNotFound)
	assert.ErrorContains(t, appendError, "append conversation turn")
}

func TestConversationRepositoryCompleteTurnWritesTheAnswerOverTheReservedRow(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns: []entities.AssistantTurn{{
			Ask: "BTCUSDT 最近走勢如何", Status: "running", CreatedAt: momentAt(10, 0),
		}},
	})
	require.NoError(t, saveError)

	completeError := conversationRepository.CompleteTurn(t.Context(), entities.AssistantTurn{
		ID:         savedConversation.Turns[0].ID,
		Answer:     "最近在盤整",
		Status:     "answered",
		Usage:      500,
		QueryCount: 1,
		Queries: []entities.AssistantQueryRecord{
			{Sequence: 1, QueryName: "list_trading_symbols", Arguments: `{}`, Outcome: `{}`},
		},
	})

	require.NoError(t, completeError)

	readBackConversation, findError := conversationRepository.FindOne(t.Context(), savedConversation.ID)
	require.NoError(t, findError)
	require.Len(t, readBackConversation.Turns, 1)
	assert.Equal(t, "最近在盤整", readBackConversation.Turns[0].Answer)
	assert.Equal(t, "answered", readBackConversation.Turns[0].Status)
	assert.Equal(t, 500, readBackConversation.Turns[0].Usage)
	// Completion must not overwrite the question or its time.
	assert.Equal(t, "BTCUSDT 最近走勢如何", readBackConversation.Turns[0].Ask)
	assert.Equal(t, momentAt(10, 0).UTC(), readBackConversation.Turns[0].CreatedAt.UTC())
	require.Len(t, readBackConversation.Turns[0].Queries, 1)
}

func TestConversationRepositoryCompleteTurnRecordsAFailureWithItsReason(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns: []entities.AssistantTurn{{
			Ask: "BTCUSDT 最近走勢如何", Status: "running", CreatedAt: momentAt(10, 0),
		}},
	})
	require.NoError(t, saveError)

	completeError := conversationRepository.CompleteTurn(t.Context(), entities.AssistantTurn{
		ID:            savedConversation.Turns[0].ID,
		Status:        "failed",
		FailureReason: "助手目前沒有回應，請稍後再試",
	})

	require.NoError(t, completeError)

	readBackConversation, findError := conversationRepository.FindOne(t.Context(), savedConversation.ID)
	require.NoError(t, findError)
	assert.Equal(t, "failed", readBackConversation.Turns[0].Status)
	assert.Equal(t, "助手目前沒有回應，請稍後再試", readBackConversation.Turns[0].FailureReason)
	assert.Empty(t, readBackConversation.Turns[0].Answer)
	assert.Equal(t, 0, readBackConversation.Turns[0].Usage)
}

func TestConversationRepositoryCompleteTurnReportsAnExchangeThatIsNotThere(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	completeError := conversationRepository.CompleteTurn(t.Context(), entities.AssistantTurn{
		ID: 9999, Status: "answered", Answer: "答案",
	})

	require.ErrorIs(t, completeError, domains.ErrConversationNotFound)
}

func TestConversationRepositoryFailAllRunningTurnsSweepsWhatAShutdownCutOff(t *testing.T) {
	// Answers are written in-process only, so a row left running after a restart must be swept.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns: []entities.AssistantTurn{
			{Ask: "答完的", Answer: "答案", Status: "answered", Usage: 100, CreatedAt: momentAt(10, 0)},
			{Ask: "還在跑的", Status: "running", CreatedAt: momentAt(10, 1)},
		},
	})
	require.NoError(t, saveError)

	sweptCount, sweepError := conversationRepository.FailAllRunningTurns(
		t.Context(), "系統重新啟動時中斷了這則回答，請再問一次")

	require.NoError(t, sweepError)
	assert.Equal(t, 1, sweptCount)

	readBackConversation, findError := conversationRepository.FindOne(t.Context(), savedConversation.ID)
	require.NoError(t, findError)
	require.Len(t, readBackConversation.Turns, 2)
	assert.Equal(t, "answered", readBackConversation.Turns[0].Status)
	assert.Equal(t, "答案", readBackConversation.Turns[0].Answer)
	assert.Equal(t, "failed", readBackConversation.Turns[1].Status)
	assert.Contains(t, readBackConversation.Turns[1].FailureReason, "重新啟動")
}

func TestConversationRepositoryFailAllRunningTurnsFindsNothingToSweepOnACleanStart(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	sweptCount, sweepError := conversationRepository.FailAllRunningTurns(t.Context(), "中斷了")

	require.NoError(t, sweepError)
	assert.Equal(t, 0, sweptCount)
}

func TestConversationRepositoryRefusesASecondRunningTurnOnOneConversation(t *testing.T) {
	// Check-then-append races, so the database index must reject a second in-flight turn.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns: []entities.AssistantTurn{{
			Ask: "第一句", Status: "running", CreatedAt: momentAt(10, 0),
		}},
	})
	require.NoError(t, saveError)

	_, appendError := conversationRepository.AppendTurn(
		t.Context(), savedConversation.ID,
		entities.AssistantTurn{Ask: "趁它還在寫再問一句", Status: "running", CreatedAt: momentAt(10, 1)})

	require.ErrorIs(t, appendError, domains.ErrAssistantAnswerInProgress)
}

func TestConversationRepositoryAcceptsTheNextQuestionOnceTheOneBeforeItHasEnded(t *testing.T) {
	// The index covers only in-flight turns, so finished turns never block a new question.
	testCases := []struct {
		name          string
		previousState string
	}{
		{name: "the one before it was answered", previousState: "answered"},
		{name: "the one before it failed", previousState: "failed"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

			savedConversation, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
				LastActiveAt: momentAt(10, 0),
				Turns: []entities.AssistantTurn{{
					Ask: "第一句", Answer: "答案", Status: testCase.previousState,
					CreatedAt: momentAt(10, 0),
				}},
			})
			require.NoError(t, saveError)

			appendedTurn, appendError := conversationRepository.AppendTurn(
				t.Context(), savedConversation.ID,
				entities.AssistantTurn{Ask: "下一句", Status: "running", CreatedAt: momentAt(10, 1)})

			require.NoError(t, appendError)
			assert.Positive(t, appendedTurn.ID)
		})
	}
}

func TestConversationRepositoryLetsTwoConversationsBeWrittenAtOnce(t *testing.T) {
	// The rule is per conversation, not per person.
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	for range 2 {
		_, saveError := conversationRepository.Save(t.Context(), entities.Conversation{
			OwnerID:      1,
			LastActiveAt: momentAt(10, 0),
			Turns: []entities.AssistantTurn{{
				Ask: "問一句", Status: "running", CreatedAt: momentAt(10, 0),
			}},
		})
		require.NoError(t, saveError)
	}
}
