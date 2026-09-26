package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
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

var connectorRouterMoment = time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)

type connectorRouterUnderTest struct {
	engine                                  *gin.Engine
	connectorClientRepository               *mocks.MockIConnectorClientRepository
	connectorAuthorizationRequestRepository *mocks.MockIConnectorAuthorizationRequestRepository
	connectorAuthorizationCodeRepository    *mocks.MockIConnectorAuthorizationCodeRepository
	sessionRepository                       *mocks.MockISessionRepository
	userRepository                          *mocks.MockIUserRepository
	accessTokenProxy                        *mocks.MockIAccessTokenProxy
	refreshTokenProxy                       *mocks.MockIRefreshTokenProxy
	opaqueIdentifierProxy                   *mocks.MockIOpaqueIdentifierProxy
}

func newConnectorRouterUnderTest(t *testing.T) connectorRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	connectorClientRepository := mocks.NewMockIConnectorClientRepository(mockController)
	connectorAuthorizationRequestRepository := mocks.NewMockIConnectorAuthorizationRequestRepository(mockController)
	connectorAuthorizationCodeRepository := mocks.NewMockIConnectorAuthorizationCodeRepository(mockController)
	sessionRepository := mocks.NewMockISessionRepository(mockController)
	userRepository := mocks.NewMockIUserRepository(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	refreshTokenProxy := mocks.NewMockIRefreshTokenProxy(mockController)
	opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(mockController)
	opaqueIdentifierProxy.EXPECT().DigestOf(gomock.Any()).DoAndReturn(func(value string) string {
		return value + "-digest"
	}).AnyTimes()
	refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).DoAndReturn(func(value string) string {
		return value + "-digest"
	}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(connectorRouterMoment).AnyTimes()
	sessionLifetimes := vo.SessionLifetimesVo{AccessToken: 15 * time.Minute, RefreshToken: 30 * 24 * time.Hour}

	connectorAuthorizationController := controller.NewConnectorAuthorizationController(
		application.NewConnectorAuthorizationApplication(
			service.NewConnectorAuthorizationService(
				connectorClientRepository, connectorAuthorizationRequestRepository,
				connectorAuthorizationCodeRepository, sessionRepository, userRepository,
				accessTokenProxy, refreshTokenProxy, opaqueIdentifierProxy, clockProxy, sessionLifetimes,
				vo.ConnectorAuthorizationPolicyVo{
					PublicBaseUrl: "https://trading-api.example.com", FrontendBaseUrl: "https://web.example.com",
					RequestLifetime: 10 * time.Minute, CodeLifetime: 5 * time.Minute,
				}),
			service.NewUserService(
				userRepository, sessionRepository, mocks.NewMockIPasswordProofProxy(mockController),
				accessTokenProxy, refreshTokenProxy, clockProxy, sessionLifetimes, testActivationPolicy,
				vo.SignInLockoutPolicyVo{FailureThreshold: 3, LockoutDuration: 7 * 24 * time.Hour}),
		))

	engine := gin.New()
	engine.GET("/.well-known/oauth-authorization-server", connectorAuthorizationController.DescribeAuthorizationServer)
	engine.POST("/oauth/register", connectorAuthorizationController.RegisterConnectorClient)
	engine.GET("/oauth/authorize", connectorAuthorizationController.StartConnectorAuthorization)
	engine.GET("/oauth/authorization-requests/:requestId", connectorAuthorizationController.GetConnectorAuthorizationRequest)
	engine.POST("/oauth/authorization-requests/:requestId/approval", doorOpenFor(t, signedInViewerID),
		connectorAuthorizationController.ApproveConnectorAuthorization)
	engine.POST("/oauth/authorization-requests/:requestId/denial", connectorAuthorizationController.DenyConnectorAuthorization)
	engine.POST("/oauth/token", connectorAuthorizationController.IssueConnectorTokens)
	engine.POST("/oauth/introspection", connectorAuthorizationController.IntrospectAccessToken)

	return connectorRouterUnderTest{
		engine:                                  engine,
		connectorClientRepository:               connectorClientRepository,
		connectorAuthorizationRequestRepository: connectorAuthorizationRequestRepository,
		connectorAuthorizationCodeRepository:    connectorAuthorizationCodeRepository,
		sessionRepository:                       sessionRepository,
		userRepository:                          userRepository,
		accessTokenProxy:                        accessTokenProxy,
		refreshTokenProxy:                       refreshTokenProxy,
		opaqueIdentifierProxy:                   opaqueIdentifierProxy,
	}
}

func (router connectorRouterUnderTest) send(request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.engine.ServeHTTP(recorder, request)

	return recorder
}

func (router connectorRouterUnderTest) postForm(target string, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return router.send(request)
}

func (router connectorRouterUnderTest) expectConnectorClientA() {
	router.connectorClientRepository.EXPECT().FindOneByClientIdentifier(gomock.Any(), "client-A").
		Return(entities.ConnectorClient{
			ClientIdentifier: "client-A", ClientName: "Claude Code",
			RedirectUris: []string{"http://localhost:33418/callback"},
		}, nil).AnyTimes()
	router.connectorClientRepository.EXPECT().FindOneByClientIdentifier(gomock.Any(), gomock.Any()).
		Return(entities.ConnectorClient{}, domains.ErrConnectorClientNotFound).AnyTimes()
}

func decodedBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	body := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	return body
}

func oauthErrorOf(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	oauthError := struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &oauthError))
	assert.NotEmpty(t, oauthError.ErrorDescription)

	return oauthError.Error
}

func TestConnectorAuthorizationControllerDescribesItselfInOAuthTerms(t *testing.T) {
	router := newConnectorRouterUnderTest(t)

	recorder := router.send(httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{
		"issuer": "https://trading-api.example.com",
		"authorization_endpoint": "https://trading-api.example.com/oauth/authorize",
		"token_endpoint": "https://trading-api.example.com/oauth/token",
		"registration_endpoint": "https://trading-api.example.com/oauth/register",
		"introspection_endpoint": "https://trading-api.example.com/oauth/introspection",
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["none"]
	}`, recorder.Body.String())
}

func TestConnectorAuthorizationControllerIgnoresTheSchemeAProxyClaims(t *testing.T) {
	router := newConnectorRouterUnderTest(t)
	request := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	request.Header.Set("X-Forwarded-Proto", "http")
	request.Header.Set("X-Forwarded-Host", "internal.example.com")

	recorder := router.send(request)

	assert.Equal(t, `"https://trading-api.example.com"`, string(decodedBody(t, recorder)["issuer"]))
}

func TestConnectorAuthorizationControllerRefusesApprovalFromAPendingAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	userRepository := mocks.NewMockIUserRepository(mockController)
	userRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.User{ID: 9, Email: "pending@example.com", IsEnabled: false}, nil)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	accessTokenProxy.EXPECT().UserIdentifiedBy("a-proof").Return(uint(9), nil)
	pendingDoor := middlewares.NewAuthenticationMiddleware(application.NewUserApplication(service.NewUserService(
		userRepository, mocks.NewMockISessionRepository(mockController), mocks.NewMockIPasswordProofProxy(mockController),
		accessTokenProxy, mocks.NewMockIRefreshTokenProxy(mockController), mocks.NewMockIClockProxy(mockController),
		vo.SessionLifetimesVo{AccessToken: 15 * time.Minute, RefreshToken: 30 * 24 * time.Hour}, testActivationPolicy,
		vo.SignInLockoutPolicyVo{FailureThreshold: 3, LockoutDuration: 7 * 24 * time.Hour}))).Handle
	engine := gin.New()
	engine.POST("/oauth/authorization-requests/:requestId/approval", pendingDoor, func(*gin.Context) {
		t.Error("a pending account must never reach the approval")
	})
	request := httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/approval", nil)
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, decodedBody(t, recorder), "activationInstruction")
}

func TestConnectorAuthorizationControllerRegistersConnectors(t *testing.T) {
	t.Run("a loopback connector is created", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "client-A"}, nil)
		router.connectorClientRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, connectorClient entities.ConnectorClient) (entities.ConnectorClient, error) {
				return connectorClient, nil
			})

		recorder := router.send(jsonRequest(http.MethodPost, "/oauth/register",
			`{"redirect_uris":["http://localhost:33418/callback"],"client_name":"Claude Code","token_endpoint_auth_method":"none"}`))

		assert.Equal(t, http.StatusCreated, recorder.Code)
		assert.JSONEq(t, `{
			"client_id": "client-A",
			"client_name": "Claude Code",
			"redirect_uris": ["http://localhost:33418/callback"],
			"grant_types": ["authorization_code", "refresh_token"],
			"response_types": ["code"],
			"token_endpoint_auth_method": "none",
			"client_id_issued_at": 1790496000
		}`, recorder.Body.String())
	})

	for _, testCase := range []struct {
		name          string
		body          string
		expectedError string
	}{
		{name: "an address elsewhere", body: `{"redirect_uris":["https://evil.example.com/cb"]}`, expectedError: "invalid_redirect_uri"},
		{name: "no address", body: `{}`, expectedError: "invalid_redirect_uri"},
		{name: "a secret-based method", body: `{"redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"client_secret_post"}`, expectedError: "invalid_client_metadata"},
		{name: "not JSON", body: `redirect_uris=http://localhost/cb`, expectedError: "invalid_client_metadata"},
	} {
		t.Run(testCase.name+" is refused", func(t *testing.T) {
			router := newConnectorRouterUnderTest(t)

			recorder := router.send(jsonRequest(http.MethodPost, "/oauth/register", testCase.body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, testCase.expectedError, oauthErrorOf(t, recorder))
		})
	}
}

func jsonRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return request
}

func authorizeTarget(change func(query url.Values)) string {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {"client-A"},
		"redirect_uri":          {"http://localhost:51000/callback"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
		"state":                 {"abc"},
		"resource":              {"https://mcp.example.com/mcp"},
		"scope":                 {"read"},
	}
	change(query)

	return "/oauth/authorize?" + query.Encode()
}

func TestConnectorAuthorizationControllerStartsAuthorization(t *testing.T) {
	t.Run("a complete request, scope and all, redirects to the web page", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectConnectorClientA()
		router.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "request-1"}, nil)
		router.connectorAuthorizationRequestRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, authorizationRequest entities.ConnectorAuthorizationRequest) (entities.ConnectorAuthorizationRequest, error) {
				assert.Equal(t, "https://mcp.example.com/mcp", authorizationRequest.Resource)
				assert.Equal(t, "abc", authorizationRequest.State)
				return authorizationRequest, nil
			})

		recorder := router.send(httptest.NewRequest(http.MethodGet, authorizeTarget(func(url.Values) {}), nil))

		assert.Equal(t, http.StatusFound, recorder.Code)
		assert.Equal(t, "https://web.example.com/connector-authorization?request=request-1", recorder.Header().Get("Location"))
	})

	for _, testCase := range []struct {
		name   string
		change func(query url.Values)
	}{
		{name: "a plain challenge method", change: func(query url.Values) { query.Set("code_challenge_method", "plain") }},
		{name: "a challenge that is not 43 base64url characters", change: func(query url.Values) {
			query.Set("code_challenge", "too-short")
		}},
	} {
		t.Run(testCase.name+" redirects back to the connector", func(t *testing.T) {
			router := newConnectorRouterUnderTest(t)
			router.expectConnectorClientA()

			recorder := router.send(httptest.NewRequest(http.MethodGet, authorizeTarget(testCase.change), nil))

			assert.Equal(t, http.StatusFound, recorder.Code)
			location, err := url.Parse(recorder.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, "localhost:51000", location.Host)
			assert.Equal(t, "invalid_request", location.Query().Get("error"))
			assert.Equal(t, "abc", location.Query().Get("state"))
		})
	}

	for _, testCase := range []struct {
		name          string
		change        func(query url.Values)
		expectedError string
	}{
		{name: "an unknown connector", change: func(query url.Values) { query.Set("client_id", "nobody") }, expectedError: "invalid_client"},
		{name: "an unregistered path", change: func(query url.Values) { query.Set("redirect_uri", "http://localhost:33418/other") }, expectedError: "invalid_request"},
	} {
		t.Run(testCase.name+" is refused without a redirect", func(t *testing.T) {
			router := newConnectorRouterUnderTest(t)
			router.expectConnectorClientA()

			recorder := router.send(httptest.NewRequest(http.MethodGet, authorizeTarget(testCase.change), nil))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Empty(t, recorder.Header().Get("Location"))
			assert.Equal(t, testCase.expectedError, oauthErrorOf(t, recorder))
		})
	}

	t.Run("a storage failure is a server error", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.connectorClientRepository.EXPECT().FindOneByClientIdentifier(gomock.Any(), gomock.Any()).
			Return(entities.ConnectorClient{}, errors.New("connection closed"))

		recorder := router.send(httptest.NewRequest(http.MethodGet, authorizeTarget(func(url.Values) {}), nil))

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Equal(t, "server_error", oauthErrorOf(t, recorder))
		assert.NotContains(t, recorder.Body.String(), "connection closed")
	})
}

func (router connectorRouterUnderTest) expectPendingRequest(createdAt time.Time) {
	router.connectorAuthorizationRequestRepository.EXPECT().FindOneByRequestIdentifier(gomock.Any(), "request-1").
		Return(entities.ConnectorAuthorizationRequest{
			ID: 21, RequestIdentifier: "request-1", ConnectorClientIdentifier: "client-A",
			RedirectUri: "http://localhost:51000/callback", State: "abc",
			ExpiresAt: createdAt.Add(10 * time.Minute),
		}, nil)
}

func TestConnectorAuthorizationControllerShowsAndDecidesRequests(t *testing.T) {
	t.Run("an open request shows its connector and expiry in camelCase", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectConnectorClientA()
		router.expectPendingRequest(connectorRouterMoment.Add(-9 * time.Minute))

		recorder := router.send(httptest.NewRequest(http.MethodGet, "/oauth/authorization-requests/request-1", nil))

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"clientName":"Claude Code","expiresAt":"2026-09-27T08:01:00Z"}`, recorder.Body.String())
	})

	t.Run("an expired request is not found", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectPendingRequest(connectorRouterMoment.Add(-10 * time.Minute))

		recorder := router.send(httptest.NewRequest(http.MethodGet, "/oauth/authorization-requests/request-1", nil))

		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Contains(t, decodedBody(t, recorder), "message")
	})

	t.Run("a signed-in user's approval returns where to send the browser", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectPendingRequest(connectorRouterMoment)
		router.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "the-code", Digest: "the-code-digest"}, nil)
		router.connectorAuthorizationRequestRepository.EXPECT().Approve(gomock.Any(), uint(21), connectorRouterMoment, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, _ time.Time, authorizationCode entities.ConnectorAuthorizationCode) error {
				assert.Equal(t, signedInViewerID, authorizationCode.UserID)
				return nil
			})
		request := httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/approval", nil)
		request.Header.Set("Authorization", signedInProof)

		recorder := router.send(request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"redirectTo":"http://localhost:51000/callback?code=the-code&state=abc"}`, recorder.Body.String())
	})

	t.Run("approving an expired request is not found", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectPendingRequest(connectorRouterMoment.Add(-10 * time.Minute))
		request := httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/approval", nil)
		request.Header.Set("Authorization", signedInProof)

		recorder := router.send(request)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("approval without signing in is refused and decides nothing", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)

		recorder := router.send(httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/approval", nil))

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("denial needs no sign-in", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectPendingRequest(connectorRouterMoment)
		router.connectorAuthorizationRequestRepository.EXPECT().Deny(gomock.Any(), uint(21), connectorRouterMoment).Return(nil)

		recorder := router.send(httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/denial", nil))

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"redirectTo":"http://localhost:51000/callback?error=access_denied&state=abc"}`, recorder.Body.String())
	})

	t.Run("a decided request cannot be denied again", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectPendingRequest(connectorRouterMoment)
		router.connectorAuthorizationRequestRepository.EXPECT().Deny(gomock.Any(), uint(21), connectorRouterMoment).
			Return(domains.ErrConnectorAuthorizationRequestNotFound)

		recorder := router.send(httptest.NewRequest(http.MethodPost, "/oauth/authorization-requests/request-1/denial", nil))

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("a storage failure is a server error that does not reveal it", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.connectorAuthorizationRequestRepository.EXPECT().FindOneByRequestIdentifier(gomock.Any(), "request-1").
			Return(entities.ConnectorAuthorizationRequest{}, errors.New("connection closed"))

		recorder := router.send(httptest.NewRequest(http.MethodGet, "/oauth/authorization-requests/request-1", nil))

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "connection closed")
	})
}

func codeExchangeForm(change func(form url.Values)) url.Values {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"the-code"},
		"redirect_uri":  {"http://localhost:51000/callback"},
		"client_id":     {"client-A"},
		"code_verifier": {"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"},
	}
	change(form)

	return form
}

func TestConnectorAuthorizationControllerIssuesTokens(t *testing.T) {
	t.Run("a valid code is exchanged for a bearer pair that must not be cached", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.expectConnectorClientA()
		router.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), "the-code-digest").
			Return(entities.ConnectorAuthorizationCode{
				ID: 31, UserID: 7, ConnectorClientIdentifier: "client-A",
				RedirectUri:   "http://localhost:51000/callback",
				CodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
				Resource:      "https://mcp.example.com/mcp", ExpiresAt: connectorRouterMoment.Add(time.Minute),
			}, nil)
		router.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{ID: 7}, nil)
		router.refreshTokenProxy.EXPECT().Mint().Return(vo.RefreshTokenVo{Value: "a-refresh-token", Digest: "d"}, nil)
		router.accessTokenProxy.EXPECT().Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: connectorRouterMoment.Add(15 * time.Minute)}, nil)
		router.connectorAuthorizationCodeRepository.EXPECT().Redeem(gomock.Any(), uint(31), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, session entities.Session) (entities.Session, error) {
				return session, nil
			})

		recorder := router.postForm("/oauth/token", codeExchangeForm(func(url.Values) {}))

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		assert.JSONEq(t, `{"access_token":"a-signed-token","token_type":"Bearer","expires_in":900,"refresh_token":"a-refresh-token"}`,
			recorder.Body.String())
	})

	t.Run("a connector renews with its renewal token", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), "a-refresh-token-digest").
			Return(entities.Session{
				ID: 41, UserID: 7, ChainID: "chain-1", ExpiresAt: connectorRouterMoment.Add(time.Hour),
				ConnectorClientIdentifier: "client-A", Audience: "https://mcp.example.com/mcp",
			}, nil)
		router.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{ID: 7}, nil)
		router.refreshTokenProxy.EXPECT().Mint().Return(vo.RefreshTokenVo{Value: "next-refresh-token", Digest: "d"}, nil)
		router.accessTokenProxy.EXPECT().Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: connectorRouterMoment.Add(15 * time.Minute)}, nil)
		router.sessionRepository.EXPECT().Rotate(gomock.Any(), uint(41), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, session entities.Session) (entities.Session, error) {
				return session, nil
			})

		recorder := router.postForm("/oauth/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {"a-refresh-token"}, "client_id": {"client-A"},
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		assert.JSONEq(t, `{"access_token":"a-signed-token","token_type":"Bearer","expires_in":900,"refresh_token":"next-refresh-token"}`,
			recorder.Body.String())
	})

	t.Run("a renewal token from another connector is an invalid grant", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{ConnectorClientIdentifier: "client-A", ExpiresAt: connectorRouterMoment.Add(time.Hour)}, nil)

		recorder := router.postForm("/oauth/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {"a-refresh-token"}, "client_id": {"client-B"},
		})

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, "invalid_grant", oauthErrorOf(t, recorder))
		assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	})

	for _, testCase := range []struct {
		name          string
		form          url.Values
		arrange       func(router connectorRouterUnderTest)
		expectedError string
	}{
		{name: "an unknown code", form: codeExchangeForm(func(url.Values) {}), expectedError: "invalid_grant", arrange: func(router connectorRouterUnderTest) {
			router.expectConnectorClientA()
			router.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
				Return(entities.ConnectorAuthorizationCode{}, domains.ErrConnectorAuthorizationCodeNotFound)
		}},
		{name: "an unknown connector", form: codeExchangeForm(func(form url.Values) { form.Set("client_id", "nobody") }), expectedError: "invalid_client", arrange: func(router connectorRouterUnderTest) {
			router.expectConnectorClientA()
		}},
		{name: "a missing verifier", form: codeExchangeForm(func(form url.Values) { form.Del("code_verifier") }), expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "a verifier shorter than 43 characters", form: codeExchangeForm(func(form url.Values) { form.Set("code_verifier", strings.Repeat("v", 42)) }), expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "a verifier longer than 128 characters", form: codeExchangeForm(func(form url.Values) { form.Set("code_verifier", strings.Repeat("v", 129)) }), expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "a verifier with a reserved character", form: codeExchangeForm(func(form url.Values) { form.Set("code_verifier", strings.Repeat("v", 42)+"/") }), expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "a missing renewal token", form: url.Values{"grant_type": {"refresh_token"}, "client_id": {"client-A"}}, expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "a missing grant type", form: url.Values{"code": {"the-code"}}, expectedError: "invalid_request", arrange: func(connectorRouterUnderTest) {}},
		{name: "the password grant", form: url.Values{"grant_type": {"password"}}, expectedError: "unsupported_grant_type", arrange: func(connectorRouterUnderTest) {}},
		{name: "an unsigned token", form: codeExchangeForm(func(url.Values) {}), expectedError: "temporarily_unavailable", arrange: func(router connectorRouterUnderTest) {
			router.expectConnectorClientA()
			router.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
				Return(entities.ConnectorAuthorizationCode{
					UserID: 7, ConnectorClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback",
					CodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", ExpiresAt: connectorRouterMoment.Add(time.Minute),
				}, nil)
			router.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{ID: 7}, nil)
			router.refreshTokenProxy.EXPECT().Mint().Return(vo.RefreshTokenVo{}, nil)
			router.accessTokenProxy.EXPECT().Issue(gomock.Any()).Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)
		}},
	} {
		t.Run(testCase.name+" answers "+testCase.expectedError, func(t *testing.T) {
			router := newConnectorRouterUnderTest(t)
			testCase.arrange(router)

			recorder := router.postForm("/oauth/token", testCase.form)

			expectedStatus := http.StatusBadRequest
			if testCase.expectedError == "temporarily_unavailable" {
				expectedStatus = http.StatusServiceUnavailable
			}
			assert.Equal(t, expectedStatus, recorder.Code)
			assert.Equal(t, testCase.expectedError, oauthErrorOf(t, recorder))
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		})
	}

	t.Run("a body that cannot be read is an invalid request", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("%zz"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := router.send(request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, "invalid_request", oauthErrorOf(t, recorder))
	})
}

func TestConnectorAuthorizationControllerIntrospectsTokens(t *testing.T) {
	t.Run("an active token", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{
			UserID: 7, Audience: "https://mcp.example.com/mcp", ExpiresAt: connectorRouterMoment.Add(15 * time.Minute),
		}, nil)
		router.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{ID: 7}, nil)

		recorder := router.postForm("/oauth/introspection", url.Values{"token": {"a-token"}})

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"active":true,"sub":"7","aud":"https://mcp.example.com/mcp","exp":1790496900}`, recorder.Body.String())
	})

	t.Run("an inactive token says nothing more", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired)

		recorder := router.postForm("/oauth/introspection", url.Values{"token": {"a-token"}})

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"active":false}`, recorder.Body.String())
	})

	t.Run("a storage failure is a server error", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		router.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{UserID: 7}, nil)
		router.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{}, errors.New("connection closed"))

		recorder := router.postForm("/oauth/introspection", url.Values{"token": {"a-token"}})

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Equal(t, "server_error", oauthErrorOf(t, recorder))
		assert.NotContains(t, recorder.Body.String(), "connection closed")
	})

	t.Run("a body that cannot be read is an invalid request", func(t *testing.T) {
		router := newConnectorRouterUnderTest(t)
		request := httptest.NewRequest(http.MethodPost, "/oauth/introspection", strings.NewReader("%zz"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := router.send(request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, "invalid_request", oauthErrorOf(t, recorder))
	})
}
