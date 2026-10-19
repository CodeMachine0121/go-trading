package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// administratorDoorFor mounts the real administrator guard; connector and browser tokens resolve alike through the token reader.
func administratorDoorFor(t *testing.T, storedUser entities.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	userRepository := mocks.NewMockIUserRepository(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	accessTokenProxy.EXPECT().UserIdentifiedBy("a-proof").Return(storedUser.ID, nil).AnyTimes()
	userRepository.EXPECT().FindOne(gomock.Any(), storedUser.ID).Return(storedUser, nil).AnyTimes()

	middleware := middlewares.NewAuthenticationMiddleware(application.NewUserApplication(service.NewUserService(
		userRepository, mocks.NewMockISessionRepository(mockController), mocks.NewMockIPasswordProofProxy(mockController),
		accessTokenProxy, mocks.NewMockIRefreshTokenProxy(mockController), mocks.NewMockIClockProxy(mockController),
		vo.SessionLifetimesVo{AccessToken: 15 * time.Minute, RefreshToken: 30 * 24 * time.Hour}, gatekeeping,
		vo.SignInLockoutPolicyVo{FailureThreshold: 3, LockoutDuration: 7 * 24 * time.Hour})))

	engine := gin.New()
	engine.GET("/screening", middleware.HandleAdministrator, func(ginContext *gin.Context) {
		ginContext.JSON(http.StatusOK, gin.H{"viewerId": middlewares.CurrentUserID(ginContext)})
	})

	return engine
}

func knockOnTheScreeningDoor(engine *gin.Engine) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/screening", nil)
	request.Header.Set("Authorization", "Bearer a-proof")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	return recorder
}

func TestTheAdministratorDoor(t *testing.T) {
	testCases := []struct {
		name         string
		user         entities.User
		expectedCode int
		expectedBody string
	}{
		{name: "an activated administrator goes in",
			user:         entities.User{ID: 7, Email: "admin@example.com", IsEnabled: true, IsAdmin: true},
			expectedCode: http.StatusOK, expectedBody: `{"viewerId":7}`},
		{name: "an activated user who is not an administrator is told it is an administrator feature",
			user:         entities.User{ID: 8, Email: "alice@example.com", IsEnabled: true},
			expectedCode: http.StatusForbidden, expectedBody: `{"message":"這是管理員功能"}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := knockOnTheScreeningDoor(administratorDoorFor(t, testCase.user))

			require.Equal(t, testCase.expectedCode, recorder.Code)
			assert.JSONEq(t, testCase.expectedBody, recorder.Body.String())
		})
	}
}

func TestTheAdministratorDoorSendsAWaitingAdministratorToActivationFirst(t *testing.T) {
	recorder := knockOnTheScreeningDoor(administratorDoorFor(t,
		entities.User{ID: 7, Email: "admin@example.com", IsAdmin: true}))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "activationInstruction")
}
