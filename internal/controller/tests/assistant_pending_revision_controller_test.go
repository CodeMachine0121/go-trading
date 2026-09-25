package controller_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var revisedSubjectLastChangedAt = time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)

type pendingRevisionRouterUnderTest struct {
	engine                    *gin.Engine
	pendingRevisionRepository *mocks.MockIAssistantPendingRevisionRepository
	applier                   *mocks.MockIAssistantRevisionApplier
}

func newPendingRevisionRouterUnderTest(t *testing.T) pendingRevisionRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	pendingRevisionRepository := mocks.NewMockIAssistantPendingRevisionRepository(mockController)
	applier := mocks.NewMockIAssistantRevisionApplier(mockController)
	applier.EXPECT().SubjectKind().Return(vo.AssistantRevisionSubjectStrategyScript).AnyTimes()

	pendingRevisionController := controller.NewAssistantPendingRevisionController(
		application.NewAssistantRevisionApplication(
			service.NewAssistantRevisionService(
				pendingRevisionRepository,
				mocks.NewMockIAssistantCreatedSubjectRepository(mockController),
				[]domaininterface.IAssistantRevisionApplier{applier},
				mocks.NewMockIClockProxy(mockController))))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/chat/pending-revisions/:id/confirm", requiresSignIn, pendingRevisionController.ConfirmPendingRevision)
	engine.POST("/chat/pending-revisions/:id/reject", requiresSignIn, pendingRevisionController.RejectPendingRevision)

	return pendingRevisionRouterUnderTest{
		engine: engine, pendingRevisionRepository: pendingRevisionRepository, applier: applier,
	}
}

func (fixture pendingRevisionRouterUnderTest) send(target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, nil)
	request.Header.Set("Authorization", signedInProof)
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func aStoredPendingRevision(status vo.AssistantPendingRevisionStatusVo) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		ID: 70, OwnerID: signedInViewerID, SubjectKind: string(vo.AssistantRevisionSubjectStrategyScript),
		SubjectID: 1, SubjectName: "二十根均線", Content: `{"strategyScriptId":1}`,
		SubjectUpdatedAt: revisedSubjectLastChangedAt, Status: string(status),
	}
}

func (fixture pendingRevisionRouterUnderTest) expectAPendingRevisionOnAnUnchangedSubject() {
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
		Return(aStoredPendingRevision(vo.AssistantPendingRevisionPending), nil).AnyTimes()
	fixture.applier.EXPECT().Inspect(gomock.Any(), signedInViewerID, `{"strategyScriptId":1}`).
		Return(dto.RewriteTargetDto{ID: 1, UpdatedAt: revisedSubjectLastChangedAt}, nil)
}

func TestPendingRevisionRouterConfirmAnswersWithTheConfirmedRevision(t *testing.T) {
	fixture := newPendingRevisionRouterUnderTest(t)
	fixture.expectAPendingRevisionOnAnUnchangedSubject()
	fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "confirmed").
		Return(true, nil)
	fixture.applier.EXPECT().Apply(gomock.Any(), signedInViewerID, `{"strategyScriptId":1}`).Return("{}", nil)

	response := fixture.send("/chat/pending-revisions/70/confirm")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"id":70,"subjectKind":"strategyScript","subjectId":1,"subjectName":"二十根均線",
		"content":{"strategyScriptId":1},"status":"confirmed","proposedAt":"0001-01-01T00:00:00Z"}`,
		response.Body.String())
}

func TestPendingRevisionRouterRejectAnswersWithTheRejectedRevision(t *testing.T) {
	fixture := newPendingRevisionRouterUnderTest(t)
	fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
		Return(aStoredPendingRevision(vo.AssistantPendingRevisionPending), nil)
	fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "rejected").
		Return(true, nil)

	response := fixture.send("/chat/pending-revisions/70/reject")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"rejected"`)
}

func TestPendingRevisionRouterIsBehindSignIn(t *testing.T) {
	fixture := newPendingRevisionRouterUnderTest(t)

	response := requestWithoutProof(fixture.engine, http.MethodPost, "/chat/pending-revisions/70/confirm", "")

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestPendingRevisionRouterMapsEachRefusalToItsStatus(t *testing.T) {
	testCases := []struct {
		name           string
		target         string
		arrange        func(fixture pendingRevisionRouterUnderTest)
		expectedStatus int
	}{
		{
			name:           "an identifier that is not a positive number",
			target:         "/chat/pending-revisions/zero/confirm",
			arrange:        func(pendingRevisionRouterUnderTest) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "a revision that is not there",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(entities.AssistantPendingRevision{}, domains.AssistantPendingRevisionNotFound(70))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "a revision already handled",
			target: "/chat/pending-revisions/70/reject",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aStoredPendingRevision(vo.AssistantPendingRevisionConfirmed), nil)
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:   "a revision whose subject changed since",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aStoredPendingRevision(vo.AssistantPendingRevisionPending), nil).AnyTimes()
				fixture.applier.EXPECT().Inspect(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(dto.RewriteTargetDto{UpdatedAt: revisedSubjectLastChangedAt.Add(time.Hour)}, nil)
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:   "a subject that is gone",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aStoredPendingRevision(vo.AssistantPendingRevisionPending), nil)
				fixture.applier.EXPECT().Inspect(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(dto.RewriteTargetDto{}, domains.StrategyScriptNotFound(1))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "a running bot holding the rewrite back",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.expectRefusedRewrite(domains.StrategyScriptBotRunning([]string{"早盤突破"}))
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:   "content the rewrite rules refuse",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.expectRefusedRewrite(domains.ErrStrategyScriptValidation)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "stored content the capability cannot read",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(70)).
					Return(aStoredPendingRevision(vo.AssistantPendingRevisionPending), nil)
				fixture.applier.EXPECT().Inspect(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(dto.RewriteTargetDto{}, domains.ErrAssistantQueryArgument)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "storage failing",
			target: "/chat/pending-revisions/70/confirm",
			arrange: func(fixture pendingRevisionRouterUnderTest) {
				fixture.expectRefusedRewrite(errors.New("storage unreachable"))
			},
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newPendingRevisionRouterUnderTest(t)
			testCase.arrange(fixture)

			response := fixture.send(testCase.target)

			assert.Equal(t, testCase.expectedStatus, response.Code)
		})
	}
}

// expectRefusedRewrite claims the revision, has the rewrite refused, and expects it handed back to pending.
func (fixture pendingRevisionRouterUnderTest) expectRefusedRewrite(refusal error) {
	fixture.expectAPendingRevisionOnAnUnchangedSubject()
	gomock.InOrder(
		fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "pending", "confirmed").
			Return(true, nil),
		fixture.pendingRevisionRepository.EXPECT().TransitionStatus(gomock.Any(), uint(70), "confirmed", "pending").
			Return(true, nil),
	)
	fixture.applier.EXPECT().Apply(gomock.Any(), gomock.Any(), gomock.Any()).Return("", refusal)
}
