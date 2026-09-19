package middlewares_test

import (
	"encoding/json"
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

// gatekeeping stands in for wherever the person running this console reads their
// mail. These tests are about which refusal comes back and what it carries, not
// about any particular inbox.
var gatekeeping = vo.AccountActivationPolicyVo{
	RequestMailbox: "gatekeeper@example.com",
	SubjectPrefix:  "console access request",
}

type doorUnderTest struct {
	engine          *gin.Engine
	handlerRunCount *int
}

// newDoorUnderTest mounts the real door in front of a handler that does nothing but
// record that it ran.
//
// What is asserted here is a property of the door, so the door is the real one and
// the only stand-ins are the two boundaries behind it: the store that holds people,
// and the thing that reads a proof.
func newDoorUnderTest(t *testing.T, storedUser entities.User, proofIsGood bool) doorUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	userRepository := mocks.NewMockIUserRepository(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)

	if proofIsGood {
		accessTokenProxy.EXPECT().UserIdentifiedBy("a-proof").
			Return(storedUser.ID, nil).AnyTimes()
		userRepository.EXPECT().FindOne(gomock.Any(), storedUser.ID).
			Return(storedUser, nil).AnyTimes()
	}

	middleware := middlewares.NewAuthenticationMiddleware(
		application.NewUserApplication(
			service.NewUserService(
				userRepository,
				mocks.NewMockISessionRepository(mockController),
				mocks.NewMockIPasswordProofProxy(mockController),
				accessTokenProxy,
				mocks.NewMockIRefreshTokenProxy(mockController),
				mocks.NewMockIClockProxy(mockController),
				vo.SessionLifetimesVo{AccessToken: 15 * time.Minute, RefreshToken: 30 * 24 * time.Hour},
				gatekeeping,
				vo.SignInLockoutPolicyVo{FailureThreshold: 3, LockoutDuration: 7 * 24 * time.Hour},
			),
		),
	)

	handlerRunCount := 0
	engine := gin.New()
	engine.GET("/behind-the-door", middleware.Handle, func(ginContext *gin.Context) {
		handlerRunCount++
		ginContext.JSON(http.StatusOK, gin.H{"viewerId": middlewares.CurrentUserID(ginContext)})
	})

	return doorUnderTest{engine: engine, handlerRunCount: &handlerRunCount}
}

func (door doorUnderTest) knock(authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/behind-the-door", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	door.engine.ServeHTTP(recorder, request)

	return recorder
}

func TestTheDoorLetsInSomebodyWhoHasBeenLetIn(t *testing.T) {
	door := newDoorUnderTest(t,
		entities.User{ID: 7, Email: "alice@example.com", IsEnabled: true}, true)

	recorder := door.knock("Bearer a-proof")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, *door.handlerRunCount)
	assert.JSONEq(t, `{"viewerId":7}`, recorder.Body.String())
}

func TestTheDoorTurnsAwaySomebodyStillWaitingToBeLetIn(t *testing.T) {
	door := newDoorUnderTest(t, entities.User{ID: 7, Email: "alice@example.com"}, true)

	recorder := door.knock("Bearer a-proof")

	// 403, not 401. Their sign-in is fine; sending them back to sign in again is the
	// one thing that cannot change their situation.
	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, 0, *door.handlerRunCount)
}

func TestTheDoorTellsAWaitingPersonWhereToWriteAndWhatToPutInTheSubject(t *testing.T) {
	door := newDoorUnderTest(t, entities.User{ID: 7, Email: "alice@example.com"}, true)

	recorder := door.knock("Bearer a-proof")

	require.Equal(t, http.StatusForbidden, recorder.Code)

	var refusal struct {
		Message               string `json:"message"`
		ActivationInstruction struct {
			RequestMailbox string `json:"requestMailbox"`
			Subject        string `json:"subject"`
		} `json:"activationInstruction"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &refusal))

	assert.Equal(t, "帳號尚未開通，請寄信申請開通", refusal.Message)
	assert.Equal(t, "gatekeeper@example.com", refusal.ActivationInstruction.RequestMailbox)
	assert.Equal(t,
		"console access request：alice@example.com", refusal.ActivationInstruction.Subject)
}

func TestTheDoorTurnsAwayAnUnprovenRequestWithADifferentAnswer(t *testing.T) {
	testCases := []struct {
		name          string
		authorization string
	}{
		{name: "no header at all", authorization: ""},
		{name: "a scheme this system does not speak", authorization: "Basic a-proof"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			door := newDoorUnderTest(t, entities.User{ID: 7}, false)

			recorder := door.knock(testCase.authorization)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Equal(t, 0, *door.handlerRunCount)
			// The two refusals stay apart, and a caller tells them apart by the
			// status alone — without reading a sentence written for a person.
			assert.JSONEq(t, `{"message":"請重新登入"}`, recorder.Body.String())
		})
	}
}

func TestTheDoorReadsTheSchemeWithoutRegardToCase(t *testing.T) {
	door := newDoorUnderTest(t,
		entities.User{ID: 7, Email: "alice@example.com", IsEnabled: true}, true)

	recorder := door.knock("bearer a-proof")

	assert.Equal(t, http.StatusOK, recorder.Code)
}
