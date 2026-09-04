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

// turnAt is one exchange, stamped so that a test about order is not also a test about
// anything else.
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
	// The lookups are never sent back to the assistant, so this record is the only
	// place that knowledge survives at all.
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
	// A chain of reasoning read out of order looks like a set of unrelated lookups.
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

	appendedConversation, appendError := conversationRepository.AppendTurn(
		t.Context(), savedConversation.ID, turnAt(momentAt(11, 0), "問 2", 200))

	require.NoError(t, appendError)
	require.Len(t, appendedConversation.Turns, 2)
	assert.Equal(t, "問 1", appendedConversation.Turns[0].Ask)
	assert.Equal(t, "問 2", appendedConversation.Turns[1].Ask)
	// When it was last active is the moment of the exchange that moved it — the same
	// fact, not a second one to keep in step.
	assert.Equal(t, momentAt(11, 0), appendedConversation.LastActiveAt.UTC())
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

func TestConversationRepositoryFindAllPutsTheMostRecentlyActiveFirst(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))
	_, oldestError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(9, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(9, 0), "舊的", 100)},
	})
	require.NoError(t, oldestError)
	_, newestError := conversationRepository.Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(12, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(12, 0), "新的", 100)},
	})
	require.NoError(t, newestError)

	conversations, findError := conversationRepository.FindAll(t.Context())

	require.NoError(t, findError)
	require.Len(t, conversations, 2)
	assert.Equal(t, momentAt(12, 0), conversations[0].LastActiveAt.UTC())
	// The exchanges come along, because the list says how many messages each holds.
	require.Len(t, conversations[0].Turns, 1)
	assert.Equal(t, "新的", conversations[0].Turns[0].Ask)
	assert.Equal(t, momentAt(9, 0), conversations[1].LastActiveAt.UTC())
}

func TestConversationRepositoryFindAllAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	conversationRepository := persistence.NewConversationRepository(newTestDatabase(t))

	conversations, findError := conversationRepository.FindAll(t.Context())

	require.NoError(t, findError)
	assert.Empty(t, conversations)
}

func TestConversationRepositorySumUsageBetweenTotalsTheStretchAcrossEveryConversation(t *testing.T) {
	// The allowance is a ceiling on the day, not on one conversation, so the total has
	// to reach across all of them.
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
	// Every read and write wraps its own failure, because "the store is down" and
	// "there is no such conversation" lead a reader to do different things, and a
	// storage failure reported as a missing conversation sends them looking for one
	// that exists.
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

	_, findAllError := conversationRepository.FindAll(t.Context())
	assert.ErrorContains(t, findAllError, "find conversations")

	_, sumError := conversationRepository.SumUsageBetween(t.Context(), momentAt(0, 0), momentAt(23, 59))
	assert.ErrorContains(t, sumError, "sum assistant usage")
}

func TestConversationRepositoryReportsAnExchangeTheStoreWillNotAccept(t *testing.T) {
	// Postgres refuses text carrying a null character. What matters is that the
	// refusal reaches the caller as a storage failure rather than as "no such
	// conversation" — the conversation is right there.
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
