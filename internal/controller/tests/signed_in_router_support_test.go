package controller_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

const signedInViewerID = uint(1)

// signedInProof's content is irrelevant; the mocked token proxy resolves it.
const signedInProof = "Bearer a-proof"

// doorOpenFor builds the real authentication middleware with a proof resolving to viewerID, so routes are proven to be actually guarded.
func doorOpenFor(t *testing.T, viewerID uint) gin.HandlerFunc {
	mockController := gomock.NewController(t)

	userRepository := mocks.NewMockIUserRepository(mockController)
	userRepository.EXPECT().FindOne(gomock.Any(), viewerID).
		Return(entities.User{
			ID: viewerID, Email: "viewer@example.com",
			// Enabled, because tests using this door are about the route; being disabled is tested beside the middleware.
			IsEnabled: true,
		}, nil).AnyTimes()

	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	accessTokenProxy.EXPECT().UserIdentifiedBy("a-proof").Return(viewerID, nil).AnyTimes()

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)).AnyTimes()

	return middlewares.NewAuthenticationMiddleware(
		application.NewUserApplication(
			service.NewUserService(
				userRepository,
				mocks.NewMockISessionRepository(mockController),
				mocks.NewMockIPasswordProofProxy(mockController),
				accessTokenProxy,
				mocks.NewMockIRefreshTokenProxy(mockController),
				clockProxy,
				vo.SessionLifetimesVo{AccessToken: 15 * time.Minute, RefreshToken: 30 * 24 * time.Hour},
				testActivationPolicy,
				vo.SignInLockoutPolicyVo{FailureThreshold: 3, LockoutDuration: 7 * 24 * time.Hour},
			),
		),
	).Handle
}

// requestWithoutProof sends what a visitor who never signed in would send.
func requestWithoutProof(engine *gin.Engine, method string, target string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	return recorder
}
