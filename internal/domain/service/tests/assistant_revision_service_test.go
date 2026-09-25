package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errRevisionStorageUnreachable = errors.New("storage unreachable")

type assistantRevisionServiceUnderTest struct {
	assistantRevisionService  *service.AssistantRevisionService
	pendingRevisionRepository *mocks.MockIAssistantPendingRevisionRepository
	createdSubjectRepository  *mocks.MockIAssistantCreatedSubjectRepository
}

func newAssistantRevisionServiceUnderTest(t *testing.T) assistantRevisionServiceUnderTest {
	controller := gomock.NewController(t)
	pendingRevisionRepository := mocks.NewMockIAssistantPendingRevisionRepository(controller)
	createdSubjectRepository := mocks.NewMockIAssistantCreatedSubjectRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)).AnyTimes()

	return assistantRevisionServiceUnderTest{
		assistantRevisionService: service.NewAssistantRevisionService(
			pendingRevisionRepository, createdSubjectRepository, clockProxy),
		pendingRevisionRepository: pendingRevisionRepository,
		createdSubjectRepository:  createdSubjectRepository,
	}
}

func aPendingRevisionOwnedBy(ownerID uint) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		ID: 70, OwnerID: ownerID, Content: `{}`,
		SubjectUpdatedAt: time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		Status:           string(vo.AssistantPendingRevisionPending),
	}
}

func TestAssistantRevisionServiceProposeReportsStorageFailures(t *testing.T) {
	t.Run("asking whether the assistant created it here", func(t *testing.T) {
		fixture := newAssistantRevisionServiceUnderTest(t)
		fixture.createdSubjectRepository.EXPECT().Exists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, errRevisionStorageUnreachable)

		_, proposeError := fixture.assistantRevisionService.Propose(t.Context(), dto.AssistantRevisionProposalDto{})

		require.ErrorIs(t, proposeError, errRevisionStorageUnreachable)
	})

	t.Run("storing the proposal", func(t *testing.T) {
		fixture := newAssistantRevisionServiceUnderTest(t)
		fixture.createdSubjectRepository.EXPECT().Exists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)
		fixture.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			Return(entities.AssistantPendingRevision{}, errRevisionStorageUnreachable)

		_, proposeError := fixture.assistantRevisionService.Propose(t.Context(), dto.AssistantRevisionProposalDto{})

		require.ErrorIs(t, proposeError, errRevisionStorageUnreachable)
	})
}

func TestAssistantRevisionServiceReportsStorageFailuresWhileActingOnAProposal(t *testing.T) {
	testCases := []struct {
		name string
		act  func(fixture assistantRevisionServiceUnderTest) error
	}{
		{
			name: "finding it",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(entities.AssistantPendingRevision{}, errRevisionStorageUnreachable)
				_, findError := fixture.assistantRevisionService.FindPendingRevision(t.Context(), 1, 70)

				return findError
			},
		},
		{
			name: "claiming it",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aPendingRevisionOwnedBy(1), nil)
				fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "confirmed").
					Return(false, errRevisionStorageUnreachable)
				_, claimError := fixture.assistantRevisionService.ClaimConfirmation(
					t.Context(), 1, 70, time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC))

				return claimError
			},
		},
		{
			name: "rejecting it",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(entities.AssistantPendingRevision{}, errRevisionStorageUnreachable)
				_, rejectError := fixture.assistantRevisionService.RejectPendingRevision(t.Context(), 1, 70)

				return rejectError
			},
		},
		{
			name: "remembering a creation",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.createdSubjectRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(errRevisionStorageUnreachable)

				return fixture.assistantRevisionService.RecordCreatedSubject(
					t.Context(), vo.AssistantQueryOriginVo{ConversationID: 5},
					vo.AssistantRevisionSubjectStrategyScript, 8)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newAssistantRevisionServiceUnderTest(t)

			assert.ErrorIs(t, testCase.act(fixture), errRevisionStorageUnreachable)
		})
	}
}

func TestAssistantRevisionServiceReopensAClaimedProposal(t *testing.T) {
	fixture := newAssistantRevisionServiceUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "confirmed", "pending").
		Return(true, nil)

	assert.NoError(t, fixture.assistantRevisionService.ReopenPendingRevision(t.Context(), 70))
}

func TestAssistantRevisionServiceRefusesAProposalForAnAnonymousViewer(t *testing.T) {
	// A proposal stored without an owner must not belong to every unauthenticated request.
	fixture := newAssistantRevisionServiceUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).Return(aPendingRevisionOwnedBy(0), nil)

	_, findError := fixture.assistantRevisionService.FindPendingRevision(t.Context(), 0, 70)

	assert.Error(t, findError)
}

func TestAssistantRevisionServiceClaimRefusesSomeoneElsesProposal(t *testing.T) {
	// No status move is stubbed: nothing may be claimed.
	fixture := newAssistantRevisionServiceUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).Return(aPendingRevisionOwnedBy(2), nil)

	_, claimError := fixture.assistantRevisionService.ClaimConfirmation(
		t.Context(), 1, 70, time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC))

	assert.Error(t, claimError)
}
