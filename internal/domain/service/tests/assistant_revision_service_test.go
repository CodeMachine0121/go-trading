package service_test

import (
	"errors"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errRevisionStorageUnreachable = errors.New("storage unreachable")

var revisedSubjectChangedAt = time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)

type assistantRevisionServiceUnderTest struct {
	assistantRevisionService  *service.AssistantRevisionService
	pendingRevisionRepository *mocks.MockIAssistantPendingRevisionRepository
	createdSubjectRepository  *mocks.MockIAssistantCreatedSubjectRepository
	applier                   *mocks.MockIAssistantRevisionApplier
}

func newAssistantRevisionServiceUnderTest(t *testing.T) assistantRevisionServiceUnderTest {
	controller := gomock.NewController(t)
	pendingRevisionRepository := mocks.NewMockIAssistantPendingRevisionRepository(controller)
	createdSubjectRepository := mocks.NewMockIAssistantCreatedSubjectRepository(controller)
	applier := mocks.NewMockIAssistantRevisionApplier(controller)
	applier.EXPECT().SubjectKind().Return(vo.AssistantRevisionSubjectStrategyScript).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)).AnyTimes()

	return assistantRevisionServiceUnderTest{
		assistantRevisionService: service.NewAssistantRevisionService(
			pendingRevisionRepository, createdSubjectRepository,
			[]domaininterface.IAssistantRevisionApplier{applier}, clockProxy),
		pendingRevisionRepository: pendingRevisionRepository,
		createdSubjectRepository:  createdSubjectRepository,
		applier:                   applier,
	}
}

func aPendingRevisionOwnedBy(ownerID uint) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		ID: 70, OwnerID: ownerID, Content: `{}`,
		SubjectKind:      string(vo.AssistantRevisionSubjectStrategyScript),
		SubjectUpdatedAt: revisedSubjectChangedAt,
		Status:           string(vo.AssistantPendingRevisionPending),
	}
}

func (fixture assistantRevisionServiceUnderTest) expectAnUnchangedSubject() {
	fixture.applier.EXPECT().Inspect(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(dto.RewriteTargetDto{ID: 1, UpdatedAt: revisedSubjectChangedAt}, nil)
}

func TestAssistantRevisionServiceReviseReportsStorageFailures(t *testing.T) {
	t.Run("asking whether the assistant created it here", func(t *testing.T) {
		fixture := newAssistantRevisionServiceUnderTest(t)
		fixture.expectAnUnchangedSubject()
		fixture.createdSubjectRepository.EXPECT().Exists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, errRevisionStorageUnreachable)

		_, reviseError := fixture.assistantRevisionService.Revise(
			t.Context(), vo.AssistantQueryOriginVo{ViewerID: 1}, vo.AssistantRevisionSubjectStrategyScript, `{}`)

		require.ErrorIs(t, reviseError, errRevisionStorageUnreachable)
	})

	t.Run("storing the proposal", func(t *testing.T) {
		fixture := newAssistantRevisionServiceUnderTest(t)
		fixture.expectAnUnchangedSubject()
		fixture.createdSubjectRepository.EXPECT().Exists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)
		fixture.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			Return(entities.AssistantPendingRevision{}, errRevisionStorageUnreachable)

		_, reviseError := fixture.assistantRevisionService.Revise(
			t.Context(), vo.AssistantQueryOriginVo{ViewerID: 1}, vo.AssistantRevisionSubjectStrategyScript, `{}`)

		require.ErrorIs(t, reviseError, errRevisionStorageUnreachable)
	})
}

func TestAssistantRevisionServiceReportsStorageFailuresWhileActingOnAProposal(t *testing.T) {
	testCases := []struct {
		name string
		act  func(fixture assistantRevisionServiceUnderTest) error
	}{
		{
			name: "finding it to confirm",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(entities.AssistantPendingRevision{}, errRevisionStorageUnreachable)
				_, confirmError := fixture.assistantRevisionService.ConfirmPendingRevision(t.Context(), 1, 70)

				return confirmError
			},
		},
		{
			name: "claiming it",
			act: func(fixture assistantRevisionServiceUnderTest) error {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aPendingRevisionOwnedBy(1), nil)
				fixture.expectAnUnchangedSubject()
				fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "confirmed").
					Return(false, errRevisionStorageUnreachable)
				_, confirmError := fixture.assistantRevisionService.ConfirmPendingRevision(t.Context(), 1, 70)

				return confirmError
			},
		},
		{
			name: "finding it to reject",
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

func TestAssistantRevisionServiceConfirmReportsARefusedRewriteEvenWhenItCannotBeReopened(t *testing.T) {
	// Both reasons reach the owner, so a proposal stuck as confirmed is not a silent failure.
	fixture := newAssistantRevisionServiceUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).Return(aPendingRevisionOwnedBy(1), nil)
	fixture.expectAnUnchangedSubject()
	fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "confirmed").
		Return(true, nil)
	fixture.applier.EXPECT().Apply(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", domains.ErrStrategyScriptNameConflict)
	fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "confirmed", "pending").
		Return(false, errRevisionStorageUnreachable)

	_, confirmError := fixture.assistantRevisionService.ConfirmPendingRevision(t.Context(), 1, 70)

	require.ErrorIs(t, confirmError, domains.ErrStrategyScriptNameConflict)
	require.ErrorIs(t, confirmError, errRevisionStorageUnreachable)
}

func TestAssistantRevisionServiceRefusesAProposalForAnAnonymousViewer(t *testing.T) {
	// A proposal stored without an owner must not belong to every unauthenticated request.
	fixture := newAssistantRevisionServiceUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).Return(aPendingRevisionOwnedBy(0), nil)

	_, confirmError := fixture.assistantRevisionService.ConfirmPendingRevision(t.Context(), 0, 70)

	require.ErrorIs(t, confirmError, domains.ErrAssistantPendingRevisionNotFound)
}
