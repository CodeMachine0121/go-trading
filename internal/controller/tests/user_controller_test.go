package controller_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var userSignInMoment = time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)

type userRouterUnderTest struct {
	engine             *gin.Engine
	userRepository     *mocks.MockIUserRepository
	sessionRepository  *mocks.MockISessionRepository
	passwordProofProxy *mocks.MockIPasswordProofProxy
	accessTokenProxy   *mocks.MockIAccessTokenProxy
	refreshTokenProxy  *mocks.MockIRefreshTokenProxy
}

func newUserRouterUnderTest(t *testing.T) userRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	userRepository := mocks.NewMockIUserRepository(mockController)
	sessionRepository := mocks.NewMockISessionRepository(mockController)
	passwordProofProxy := mocks.NewMockIPasswordProofProxy(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	refreshTokenProxy := mocks.NewMockIRefreshTokenProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(userSignInMoment).AnyTimes()

	userController := controller.NewUserController(
		application.NewUserApplication(
			service.NewUserService(
				userRepository, sessionRepository, passwordProofProxy, accessTokenProxy,
				refreshTokenProxy, clockProxy, vo.SessionLifetimesVo{
					AccessToken:  15 * time.Minute,
					RefreshToken: 30 * 24 * time.Hour,
				})))

	engine := gin.New()
	engine.POST("/users", userController.RegisterUser)
	engine.POST("/sessions", userController.SignIn)
	engine.POST("/sessions/renewal", userController.RenewSession)
	engine.POST("/sessions/revocation", userController.RevokeSession)
	engine.GET("/users/me", userController.GetCurrentUser)

	return userRouterUnderTest{
		engine:             engine,
		userRepository:     userRepository,
		sessionRepository:  sessionRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
		refreshTokenProxy:  refreshTokenProxy,
	}
}

// expectSessionOpened sets up everything opening a session touches, for the tests
// that are about the response rather than about how it got there.
func (fixture userRouterUnderTest) expectSessionOpened() {
	fixture.refreshTokenProxy.EXPECT().
		Mint().
		Return(vo.RefreshTokenVo{Value: "a-refresh-token", Digest: "a-digest"}, nil)
	fixture.accessTokenProxy.EXPECT().
		Issue(gomock.Any(), gomock.Any()).
		Return(vo.AccessTokenVo{
			AccessToken: "a-signed-token",
			ExpiresAt:   time.Date(2026, 9, 5, 8, 15, 0, 0, time.UTC),
		}, nil)
	fixture.sessionRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
			session.ID = 11
			session.ExpiresAt = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)

			return session, nil
		})
}

func (fixture userRouterUnderTest) send(
	method string, target string, body string, authorization string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

const aCredentialsBody = `{"email":"james@example.com","password":"correct horse"}`

func aStoredUserRow(id uint) entities.User {
	return entities.User{
		ID:            id,
		Email:         "james@example.com",
		PasswordProof: "a-password-proof",
		CreatedAt:     userSignInMoment,
		UpdatedAt:     userSignInMoment,
	}
}

func TestUserRouterRegisterUser(t *testing.T) {
	t.Run("answers created, and the answer carries no trace of the password", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(aStoredUserRow(7), nil)

		recorder := fixture.send(http.MethodPost, "/users", aCredentialsBody, "")

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.JSONEq(t, `{"id":7,"email":"james@example.com"}`, recorder.Body.String())
	})

	t.Run("answers bad request for a body that is not readable", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)

		recorder := fixture.send(http.MethodPost, "/users", `{"email":`, "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("answers bad request when a rule was broken", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)

		recorder := fixture.send(
			http.MethodPost, "/users", `{"email":"not-an-email","password":"correct horse"}`, "")

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "格式")
	})

	t.Run("answers conflict when somebody already holds that address", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.EmailAlreadyRegistered("james@example.com"))

		recorder := fixture.send(http.MethodPost, "/users", aCredentialsBody, "")

		assert.Equal(t, http.StatusConflict, recorder.Code)
	})

	t.Run("answers bad gateway when storage broke", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.User{}, errors.New("save user: connection closed"))

		recorder := fixture.send(http.MethodPost, "/users", aCredentialsBody, "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestUserRouterSignIn(t *testing.T) {
	t.Run("answers with both proofs and both moments", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), "james@example.com").
			Return(aStoredUserRow(7), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.expectSessionOpened()

		recorder := fixture.send(http.MethodPost, "/sessions", aCredentialsBody, "")

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{
			"accessToken": "a-signed-token",
			"expiresAt": "2026-09-05T08:15:00Z",
			"refreshToken": "a-refresh-token",
			"refreshTokenExpiresAt": "2026-10-05T08:00:00Z"
		}`, recorder.Body.String())
	})

	t.Run("answers bad request for a body that is not readable", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)

		recorder := fixture.send(http.MethodPost, "/sessions", `{"email":`, "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("answers unauthorized with the one sentence a failed sign-in has", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUserRow(7), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		recorder := fixture.send(http.MethodPost, "/sessions", aCredentialsBody, "")

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.JSONEq(t, `{"message":"電子郵件或密碼不正確"}`, recorder.Body.String())
	})

	t.Run("answers service unavailable when there is no key to sign with", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUserRow(7), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-refresh-token", Digest: "a-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
			Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)

		recorder := fixture.send(http.MethodPost, "/sessions", aCredentialsBody, "")

		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code,
			"密碼是對的，改什麼都沒用——不能說成「你送錯了」")
	})

	t.Run("answers bad gateway when storage broke", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(entities.User{}, errors.New("find user by email: connection closed"))

		recorder := fixture.send(http.MethodPost, "/sessions", aCredentialsBody, "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestUserRouterGetCurrentUser(t *testing.T) {
	t.Run("answers with who the proof belongs to", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy("a-signed-token").
			Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUserRow(7), nil)

		recorder := fixture.send(http.MethodGet, "/users/me", "", "Bearer a-signed-token")

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"id":7,"email":"james@example.com"}`, recorder.Body.String())
	})

	t.Run("reads the scheme without regard to case, because HTTP says so", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy("a-signed-token").
			Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUserRow(7), nil)

		recorder := fixture.send(http.MethodGet, "/users/me", "", "bearer a-signed-token")

		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("turns away a request presenting nothing", func(t *testing.T) {
		testCases := []struct {
			name          string
			authorization string
		}{
			{name: "no header at all", authorization: ""},
			{name: "a proof with no scheme in front of it", authorization: "a-signed-token"},
			{name: "the scheme with nothing after it", authorization: "Bearer "},
			{name: "some other scheme entirely", authorization: "Basic YWJjOmRlZg=="},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				// Nothing is set up on any mock: a request presenting no proof must
				// be turned away before anything downstream is asked anything.
				fixture := newUserRouterUnderTest(t)

				recorder := fixture.send(
					http.MethodGet, "/users/me", "", testCase.authorization)

				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.Contains(t, recorder.Body.String(), "重新登入")
			})
		}
	})

	t.Run("answers unauthorized for a proof that is not one", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy(gomock.Any()).
			Return(uint(0), domains.ErrAuthenticationRequired)

		recorder := fixture.send(http.MethodGet, "/users/me", "", "Bearer a-tampered-token")

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("answers bad gateway when storage broke", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, errors.New("find user: connection closed"))

		recorder := fixture.send(http.MethodGet, "/users/me", "", "Bearer a-signed-token")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

const aRenewalBody = `{"refreshToken":"a-refresh-token"}`

// aStoredSessionRow is a session as storage hands it back, good for another month.
func aStoredSessionRow() entities.Session {
	return entities.Session{
		ID:                 11,
		UserID:             7,
		ChainID:            "a-chain",
		RefreshTokenDigest: "a-digest",
		ExpiresAt:          time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC),
		CreatedAt:          userSignInMoment,
	}
}

func TestUserRouterRenewSession(t *testing.T) {
	t.Run("answers with a fresh pair", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf("a-refresh-token").Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-digest").
			Return(aStoredSessionRow(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUserRow(7), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
			Return(vo.AccessTokenVo{
				AccessToken: "a-newer-signed-token",
				ExpiresAt:   time.Date(2026, 9, 5, 8, 15, 0, 0, time.UTC),
			}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), uint(11), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, next entities.Session) (entities.Session, error) {
				next.ID = 12
				return next, nil
			})

		recorder := fixture.send(http.MethodPost, "/sessions/renewal", aRenewalBody, "")

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{
			"accessToken": "a-newer-signed-token",
			"expiresAt": "2026-09-05T08:15:00Z",
			"refreshToken": "a-newer-token",
			"refreshTokenExpiresAt": "2026-10-05T08:00:00Z"
		}`, recorder.Body.String())
	})

	t.Run("answers bad request for a body that is not readable", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)

		recorder := fixture.send(http.MethodPost, "/sessions/renewal", `{"refreshToken":`, "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("answers unauthorized with the one sentence a failed renewal has", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionNotFound)

		recorder := fixture.send(http.MethodPost, "/sessions/renewal", aRenewalBody, "")

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.JSONEq(t, `{"message":"請重新登入"}`, recorder.Body.String())
	})

	t.Run("answers service unavailable when there is no key to sign with", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSessionRow(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUserRow(7), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
			Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)

		recorder := fixture.send(http.MethodPost, "/sessions/renewal", aRenewalBody, "")

		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	})

	t.Run("answers bad gateway when storage broke", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, errors.New("find session: connection closed"))

		recorder := fixture.send(http.MethodPost, "/sessions/renewal", aRenewalBody, "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestUserRouterRevokeSession(t *testing.T) {
	t.Run("answers no content and says nothing else", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf("a-refresh-token").Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-digest").
			Return(aStoredSessionRow(), nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		recorder := fixture.send(http.MethodPost, "/sessions/revocation", aRenewalBody, "")

		require.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Empty(t, recorder.Body.String())
	})

	t.Run("answers no content even when there was nothing to end", func(t *testing.T) {
		// The caller asked for this sign-in to stop working. It already does not
		// work. Telling them otherwise would have them retrying to reach a state
		// they are already in.
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionNotFound)

		recorder := fixture.send(http.MethodPost, "/sessions/revocation", aRenewalBody, "")

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("answers bad request for a body that is not readable", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)

		recorder := fixture.send(http.MethodPost, "/sessions/revocation", `{"refreshToken":`, "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("answers bad gateway when storage broke", func(t *testing.T) {
		fixture := newUserRouterUnderTest(t)
		fixture.refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).Return("a-digest")
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, errors.New("find session: connection closed"))

		recorder := fixture.send(http.MethodPost, "/sessions/revocation", aRenewalBody, "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
