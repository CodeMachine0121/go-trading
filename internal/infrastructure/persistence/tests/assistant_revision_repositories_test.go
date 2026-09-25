package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aConversationWithOneTurn stores the conversation and exchange a proposal belongs to.
func aConversationWithOneTurn(t *testing.T, database *gorm.DB) entities.Conversation {
	savedConversation, saveError := persistence.NewConversationRepository(database).Save(t.Context(), entities.Conversation{
		LastActiveAt: momentAt(10, 0),
		Turns:        []entities.AssistantTurn{turnAt(momentAt(10, 0), "幫我把均線改成六十根", 100)},
	})
	require.NoError(t, saveError)

	return savedConversation
}

func aPendingRevisionIn(conversation entities.Conversation) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		OwnerID:          1,
		ConversationID:   conversation.ID,
		AssistantTurnID:  conversation.Turns[0].ID,
		SubjectKind:      string(vo.AssistantRevisionSubjectStrategyScript),
		SubjectID:        7,
		SubjectName:      "二十根均線",
		Content:          `{"strategyScriptId":7,"name":"六十根均線"}`,
		SubjectUpdatedAt: time.Date(2026, 9, 3, 8, 0, 0, 123456000, time.UTC),
		Status:           string(vo.AssistantPendingRevisionPending),
		ProposedAt:       momentAt(10, 1),
	}
}

func TestAssistantPendingRevisionRepositoryReadsBackWhatWasProposed(t *testing.T) {
	database := newTestDatabase(t)
	repository := persistence.NewAssistantPendingRevisionRepository(database)
	proposal := aPendingRevisionIn(aConversationWithOneTurn(t, database))

	savedRevision, saveError := repository.Save(t.Context(), proposal)
	require.NoError(t, saveError)
	require.Positive(t, savedRevision.ID)

	readRevision, findError := repository.FindOne(t.Context(), savedRevision.ID)
	require.NoError(t, findError)

	assert.Equal(t, proposal.Content, readRevision.Content)
	assert.Equal(t, proposal.SubjectName, readRevision.SubjectName)
	// The staleness check compares instants, so the stored one must be the same instant.
	assert.True(t, proposal.SubjectUpdatedAt.Equal(readRevision.SubjectUpdatedAt))
}

func TestAssistantPendingRevisionRepositoryAnswersAMissingOneAsNotFound(t *testing.T) {
	repository := persistence.NewAssistantPendingRevisionRepository(newTestDatabase(t))

	_, findError := repository.FindOne(t.Context(), 999999)

	require.ErrorIs(t, findError, domains.ErrAssistantPendingRevisionNotFound)
}

func TestAssistantPendingRevisionRepositoryMovesAStatusOnlyFromWhereItStands(t *testing.T) {
	database := newTestDatabase(t)
	repository := persistence.NewAssistantPendingRevisionRepository(database)
	savedRevision, saveError := repository.Save(t.Context(), aPendingRevisionIn(aConversationWithOneTurn(t, database)))
	require.NoError(t, saveError)

	firstPress, firstError := repository.TransitionStatus(t.Context(), savedRevision.ID, "pending", "confirmed")
	secondPress, secondError := repository.TransitionStatus(t.Context(), savedRevision.ID, "pending", "confirmed")

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.True(t, firstPress)
	assert.False(t, secondPress)
}

func TestConversationRepositoryReadsEachExchangesProposalsWithIt(t *testing.T) {
	database := newTestDatabase(t)
	conversation := aConversationWithOneTurn(t, database)
	repository := persistence.NewAssistantPendingRevisionRepository(database)
	for _, subjectID := range []uint{7, 8} {
		proposal := aPendingRevisionIn(conversation)
		proposal.SubjectID = subjectID
		_, saveError := repository.Save(t.Context(), proposal)
		require.NoError(t, saveError)
	}

	readConversation, findError := persistence.NewConversationRepository(database).FindOne(t.Context(), conversation.ID)
	require.NoError(t, findError)

	require.Len(t, readConversation.Turns[0].PendingRevisions, 2)
	assert.Equal(t, uint(7), readConversation.Turns[0].PendingRevisions[0].SubjectID)
	assert.Equal(t, uint(8), readConversation.Turns[0].PendingRevisions[1].SubjectID)
}

func TestAssistantCreatedSubjectRepositoryRemembersPerConversation(t *testing.T) {
	database := newTestDatabase(t)
	repository := persistence.NewAssistantCreatedSubjectRepository(database)
	conversation := aConversationWithOneTurn(t, database)
	otherConversation := aConversationWithOneTurn(t, database)
	createdSubject := entities.AssistantCreatedSubject{
		ConversationID: conversation.ID, SubjectKind: "strategyScript", SubjectID: 7, CreatedAt: momentAt(10, 2),
	}

	require.NoError(t, repository.Save(t.Context(), createdSubject))
	// Remembering it twice is harmless.
	require.NoError(t, repository.Save(t.Context(), createdSubject))

	createdHere, hereError := repository.Exists(t.Context(), conversation.ID, "strategyScript", 7)
	createdElsewhere, elsewhereError := repository.Exists(t.Context(), otherConversation.ID, "strategyScript", 7)
	otherKind, kindError := repository.Exists(t.Context(), conversation.ID, "tradingStrategy", 7)

	require.NoError(t, hereError)
	require.NoError(t, elsewhereError)
	require.NoError(t, kindError)
	assert.True(t, createdHere)
	assert.False(t, createdElsewhere)
	assert.False(t, otherKind)
}
