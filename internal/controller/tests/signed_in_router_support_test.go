package controller_test

import (
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

// signedInViewerID is whoever every guarded route below is reached as.
const signedInViewerID = uint(1)

// signedInProof is the header value those requests carry. Its content does not
// matter — what turns it into a person is the token proxy, which is mocked.
const signedInProof = "Bearer a-proof"

// doorOpenFor builds the real authentication middleware with a proof that resolves
// to this person.
//
// The real door is used rather than a stand-in that plants a user directly, because
// a stand-in would let these tests pass on a route nobody had actually guarded. The
// two things being asserted here — that a guarded route knows who is asking, and
// that an unproven request never reaches the handler — are properties of the door,
// so the door has to be in the picture.
func doorOpenFor(t *testing.T, viewerID uint) gin.HandlerFunc {
	mockController := gomock.NewController(t)

	userRepository := mocks.NewMockIUserRepository(mockController)
	userRepository.EXPECT().FindOne(gomock.Any(), viewerID).
		Return(entities.User{ID: viewerID, Email: "viewer@example.com"}, nil).AnyTimes()

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
			),
		),
	).Handle
}
