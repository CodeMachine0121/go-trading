package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aPendingRevisionRow(id uint, status vo.AssistantPendingRevisionStatusVo) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		ID:          id,
		SubjectKind: string(vo.AssistantRevisionSubjectStrategyScript),
		SubjectID:   1,
		SubjectName: "二十根均線",
		Content:     `{"strategyScriptId":1,"name":"六十根均線"}`,
		Status:      string(status),
		ProposedAt:  time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC),
	}
}

func TestConversationDomainToDtoCarriesEachExchangesProposalsOnItsLastMessage(t *testing.T) {
	testCases := []struct {
		name                    string
		status                  string
		expectedMessageCount    int
		expectedCarryingMessage int
	}{
		{name: "an answered exchange carries them on its answer", status: "answered",
			expectedMessageCount: 2, expectedCarryingMessage: 1},
		{name: "a failed exchange keeps them on its question", status: "failed",
			expectedMessageCount: 1, expectedCarryingMessage: 0},
		{name: "a running exchange shows them on its question already", status: "running",
			expectedMessageCount: 1, expectedCarryingMessage: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conversation := entities.Conversation{Turns: []entities.AssistantTurn{{
				Ask: "幫我把均線改成六十根", Answer: "已提出", Status: testCase.status,
				PendingRevisions: []entities.AssistantPendingRevision{
					aPendingRevisionRow(70, vo.AssistantPendingRevisionPending),
					aPendingRevisionRow(71, vo.AssistantPendingRevisionConfirmed),
				},
			}}}

			messages := domains.NewConversationDomain(conversation).ToDto().Messages

			require.Len(t, messages, testCase.expectedMessageCount)
			for index, message := range messages {
				if index != testCase.expectedCarryingMessage {
					assert.Empty(t, message.PendingRevisions)
					continue
				}

				require.Len(t, message.PendingRevisions, 2)
				assert.Equal(t, uint(70), message.PendingRevisions[0].ID)
				assert.Equal(t, "strategyScript", message.PendingRevisions[0].SubjectKind)
				assert.Equal(t, "二十根均線", message.PendingRevisions[0].SubjectName)
				assert.JSONEq(t, `{"strategyScriptId":1,"name":"六十根均線"}`, string(message.PendingRevisions[0].Content))
				assert.Equal(t, "pending", message.PendingRevisions[0].Status)
				assert.Equal(t, time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC), message.PendingRevisions[0].ProposedAt)
				assert.Equal(t, "confirmed", message.PendingRevisions[1].Status)
			}
		})
	}
}

func TestConversationDomainToDtoCarriesNoProposalsWhenThereAreNone(t *testing.T) {
	conversationDto := domains.NewConversationDomain(conversationOf(1)).ToDto()

	for _, message := range conversationDto.Messages {
		assert.Empty(t, message.PendingRevisions)
	}
}

func TestAssistantRevisionProposalDomainWritesNowOnlyWhatTheAssistantCreatedHereAndNoBotUses(t *testing.T) {
	testCases := []struct {
		name                    string
		createdInConversation   bool
		botReferenceCount       int
		expectedAppliesDirectly bool
	}{
		{name: "created here and used by no bot", createdInConversation: true, botReferenceCount: 0, expectedAppliesDirectly: true},
		{name: "created here but used by a bot", createdInConversation: true, botReferenceCount: 1, expectedAppliesDirectly: false},
		{name: "not created here", createdInConversation: false, botReferenceCount: 0, expectedAppliesDirectly: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proposal := aProposal(testCase.botReferenceCount)

			proposalDomain := domains.NewAssistantRevisionProposalDomain(
				proposal, testCase.createdInConversation, time.Now())

			assert.Equal(t, testCase.expectedAppliesDirectly, proposalDomain.AppliesDirectly())
		})
	}
}

func aProposal(botReferenceCount int) dto.AssistantRevisionProposalDto {
	return dto.AssistantRevisionProposalDto{
		ViewerID: 1, ConversationID: 5, TurnID: 50, SubjectKind: "strategyScript",
		Target:  dto.RewriteTargetDto{ID: 1, Name: "二十根均線", BotReferenceCount: botReferenceCount},
		Content: `{}`,
	}
}
