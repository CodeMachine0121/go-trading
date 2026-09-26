package application_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
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

var connectorMoment = time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)

const (
	connectorCodeVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	connectorCodeChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	connectorResource      = "https://mcp.example.com/mcp"
)

var connectorAuthorizationPolicy = vo.ConnectorAuthorizationPolicyVo{
	PublicBaseUrl:   "https://trading-api.example.com",
	FrontendBaseUrl: "https://web.example.com",
	RequestLifetime: 10 * time.Minute,
	CodeLifetime:    5 * time.Minute,
}

type connectorAuthorizationApplicationUnderTest struct {
	connectorAuthorizationApplication       *application.ConnectorAuthorizationApplication
	userApplication                         *application.UserApplication
	connectorClientRepository               *mocks.MockIConnectorClientRepository
	connectorAuthorizationRequestRepository *mocks.MockIConnectorAuthorizationRequestRepository
	connectorAuthorizationCodeRepository    *mocks.MockIConnectorAuthorizationCodeRepository
	sessionRepository                       *mocks.MockISessionRepository
	userRepository                          *mocks.MockIUserRepository
	accessTokenProxy                        *mocks.MockIAccessTokenProxy
	refreshTokenProxy                       *mocks.MockIRefreshTokenProxy
	opaqueIdentifierProxy                   *mocks.MockIOpaqueIdentifierProxy
}

// newConnectorAuthorizationApplicationUnderTest uses the real domain services, mocking only stores, token proxies and the clock.
func newConnectorAuthorizationApplicationUnderTest(t *testing.T) connectorAuthorizationApplicationUnderTest {
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
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(connectorMoment).AnyTimes()
	refreshTokenProxy.EXPECT().DigestOf(gomock.Any()).DoAndReturn(func(value string) string {
		return value + "-digest"
	}).AnyTimes()

	userService := service.NewUserService(
		userRepository, sessionRepository, mocks.NewMockIPasswordProofProxy(mockController),
		accessTokenProxy, refreshTokenProxy, clockProxy, sessionLifetimes, activationPolicy, lockoutPolicy)

	return connectorAuthorizationApplicationUnderTest{
		connectorAuthorizationApplication: application.NewConnectorAuthorizationApplication(
			service.NewConnectorAuthorizationService(
				connectorClientRepository, connectorAuthorizationRequestRepository,
				connectorAuthorizationCodeRepository, sessionRepository, userRepository,
				accessTokenProxy, refreshTokenProxy, opaqueIdentifierProxy, clockProxy, sessionLifetimes,
				connectorAuthorizationPolicy),
			userService),
		userApplication:                         application.NewUserApplication(userService),
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

func aConnectorClient(clientIdentifier string) entities.ConnectorClient {
	return entities.ConnectorClient{
		ID: 3, ClientIdentifier: clientIdentifier, ClientName: "Claude Code",
		RedirectUris: []string{"http://localhost:33418/callback"},
	}
}

func (fixture connectorAuthorizationApplicationUnderTest) expectConnectorClients(clientIdentifiers ...string) {
	for _, clientIdentifier := range clientIdentifiers {
		fixture.connectorClientRepository.EXPECT().
			FindOneByClientIdentifier(gomock.Any(), clientIdentifier).
			Return(aConnectorClient(clientIdentifier), nil).AnyTimes()
	}
	fixture.connectorClientRepository.EXPECT().
		FindOneByClientIdentifier(gomock.Any(), gomock.Any()).
		Return(entities.ConnectorClient{}, domains.ErrConnectorClientNotFound).AnyTimes()
}

func TestConnectorAuthorizationApplicationDescribesItselfAtThePublicAddress(t *testing.T) {
	fixture := newConnectorAuthorizationApplicationUnderTest(t)

	metadata := fixture.connectorAuthorizationApplication.DescribeAuthorizationServer()

	assert.Equal(t, dto.ConnectorAuthorizationServerMetadataDto{
		Issuer:                            "https://trading-api.example.com",
		AuthorizationEndpoint:             "https://trading-api.example.com/oauth/authorize",
		TokenEndpoint:                     "https://trading-api.example.com/oauth/token",
		RegistrationEndpoint:              "https://trading-api.example.com/oauth/register",
		IntrospectionEndpoint:             "https://trading-api.example.com/oauth/introspection",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}, metadata)
}

func TestConnectorAuthorizationApplicationRegisterConnectorClient(t *testing.T) {
	t.Run("a loopback connector gets a fresh identifier and the fixed capabilities", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "client-A", Digest: "unused"}, nil)
		fixture.connectorClientRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, connectorClient entities.ConnectorClient) (entities.ConnectorClient, error) {
				connectorClient.ID = 3
				return connectorClient, nil
			})

		connectorClientDto, err := fixture.connectorAuthorizationApplication.RegisterConnectorClient(
			t.Context(), dto.ConnectorClientRegistrationDto{
				RedirectUris: []string{"http://localhost:33418/callback"}, ClientName: "Claude Code",
			})

		require.NoError(t, err)
		assert.Equal(t, dto.ConnectorClientDto{
			ClientIdentifier:         "client-A",
			ClientName:               "Claude Code",
			RedirectUris:             []string{"http://localhost:33418/callback"},
			GrantTypes:               []string{"authorization_code", "refresh_token"},
			ResponseTypes:            []string{"code"},
			TokenEndpointAuthMethod:  "none",
			ClientIdentifierIssuedAt: connectorMoment.Unix(),
		}, connectorClientDto)
	})

	t.Run("an address elsewhere is refused before anything is minted or stored", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)

		_, err := fixture.connectorAuthorizationApplication.RegisterConnectorClient(
			t.Context(), dto.ConnectorClientRegistrationDto{RedirectUris: []string{"https://evil.example.com/cb"}})

		require.ErrorIs(t, err, domains.ErrConnectorRedirectUriInvalid)
	})

	t.Run("failures to mint or store are passed on", func(t *testing.T) {
		mintFailure := errors.New("no randomness")
		storageFailure := errors.New("connection closed")
		for _, testCase := range []struct {
			name          string
			arrange       func(fixture connectorAuthorizationApplicationUnderTest)
			expectedError error
		}{
			{name: "mint", expectedError: mintFailure, arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
				fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{}, mintFailure)
			}},
			{name: "store", expectedError: storageFailure, arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
				fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "client-A"}, nil)
				fixture.connectorClientRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.ConnectorClient{}, storageFailure)
			}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newConnectorAuthorizationApplicationUnderTest(t)
				testCase.arrange(fixture)

				_, err := fixture.connectorAuthorizationApplication.RegisterConnectorClient(
					t.Context(), dto.ConnectorClientRegistrationDto{RedirectUris: []string{"http://localhost/cb"}})

				require.ErrorIs(t, err, testCase.expectedError)
			})
		}
	})
}

func aStartDto() dto.ConnectorAuthorizationStartDto {
	return dto.ConnectorAuthorizationStartDto{
		ResponseType: "code", ClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback",
		CodeChallenge: connectorCodeChallenge, CodeChallengeMethod: "S256", State: "abc", Resource: connectorResource,
	}
}

func TestConnectorAuthorizationApplicationStartConnectorAuthorization(t *testing.T) {
	t.Run("a complete request is recorded as sent and the browser goes to the web page", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "request-1"}, nil)
		fixture.connectorAuthorizationRequestRepository.EXPECT().
			Save(gomock.Any(), entities.ConnectorAuthorizationRequest{
				RequestIdentifier:         "request-1",
				ConnectorClientIdentifier: "client-A",
				RedirectUri:               "http://localhost:51000/callback",
				CodeChallenge:             connectorCodeChallenge,
				State:                     "abc",
				Resource:                  connectorResource,
				ExpiresAt:                 connectorMoment.Add(10 * time.Minute),
				CreatedAt:                 connectorMoment,
			}).
			DoAndReturn(func(_ context.Context, authorizationRequest entities.ConnectorAuthorizationRequest) (entities.ConnectorAuthorizationRequest, error) {
				return authorizationRequest, nil
			})

		redirect, err := fixture.connectorAuthorizationApplication.StartConnectorAuthorization(t.Context(), aStartDto())

		require.NoError(t, err)
		assert.Equal(t, "https://web.example.com/connector-authorization?request=request-1", redirect.RedirectTo)
	})

	t.Run("an untrusted connector or address is refused without any redirect", func(t *testing.T) {
		for _, testCase := range []struct {
			name          string
			change        func(startDto *dto.ConnectorAuthorizationStartDto)
			expectedError error
		}{
			{name: "unknown connector", change: func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.ClientIdentifier = "nobody" }, expectedError: domains.ErrConnectorClientNotFound},
			{name: "another path", change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.RedirectUri = "http://localhost:33418/other"
			}, expectedError: domains.ErrConnectorRedirectUriNotRegistered},
			{name: "no address", change: func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.RedirectUri = "" }, expectedError: domains.ErrConnectorRedirectUriNotRegistered},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newConnectorAuthorizationApplicationUnderTest(t)
				fixture.expectConnectorClients("client-A")
				startDto := aStartDto()
				testCase.change(&startDto)

				redirect, err := fixture.connectorAuthorizationApplication.StartConnectorAuthorization(t.Context(), startDto)

				require.ErrorIs(t, err, testCase.expectedError)
				assert.Empty(t, redirect.RedirectTo)
			})
		}
	})

	t.Run("an incomplete request goes back to the connector and records nothing", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		startDto := aStartDto()
		startDto.CodeChallenge = ""

		redirect, err := fixture.connectorAuthorizationApplication.StartConnectorAuthorization(t.Context(), startDto)

		require.NoError(t, err)
		sentBack, parseError := url.Parse(redirect.RedirectTo)
		require.NoError(t, parseError)
		assert.Equal(t, "localhost:51000", sentBack.Host)
		assert.Equal(t, "/callback", sentBack.Path)
		assert.Equal(t, "invalid_request", sentBack.Query().Get("error"))
		assert.Equal(t, "abc", sentBack.Query().Get("state"))
	})

	t.Run("failures to mint or store are passed on", func(t *testing.T) {
		mintFailure := errors.New("no randomness")
		storageFailure := errors.New("connection closed")
		for _, testCase := range []struct {
			name          string
			arrange       func(fixture connectorAuthorizationApplicationUnderTest)
			expectedError error
		}{
			{name: "mint", expectedError: mintFailure, arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
				fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{}, mintFailure)
			}},
			{name: "store", expectedError: storageFailure, arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
				fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "request-1"}, nil)
				fixture.connectorAuthorizationRequestRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.ConnectorAuthorizationRequest{}, storageFailure)
			}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newConnectorAuthorizationApplicationUnderTest(t)
				fixture.expectConnectorClients("client-A")
				testCase.arrange(fixture)

				_, err := fixture.connectorAuthorizationApplication.StartConnectorAuthorization(t.Context(), aStartDto())

				require.ErrorIs(t, err, testCase.expectedError)
			})
		}
	})
}

func aPendingAuthorizationRequest(createdAt time.Time, decidedAt *time.Time) entities.ConnectorAuthorizationRequest {
	return entities.ConnectorAuthorizationRequest{
		ID: 21, RequestIdentifier: "request-1", ConnectorClientIdentifier: "client-A",
		RedirectUri: "http://localhost:51000/callback", CodeChallenge: connectorCodeChallenge,
		State: "abc", Resource: connectorResource,
		ExpiresAt: createdAt.Add(10 * time.Minute), DecidedAt: decidedAt, CreatedAt: createdAt,
	}
}

func (fixture connectorAuthorizationApplicationUnderTest) expectAuthorizationRequest(
	authorizationRequest entities.ConnectorAuthorizationRequest,
) {
	fixture.connectorAuthorizationRequestRepository.EXPECT().
		FindOneByRequestIdentifier(gomock.Any(), "request-1").Return(authorizationRequest, nil)
}

func TestConnectorAuthorizationApplicationGetConnectorAuthorizationRequest(t *testing.T) {
	decidedAt := connectorMoment.Add(-time.Minute)

	t.Run("a nine-minute-old request shows its connector and expiry", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment.Add(-9*time.Minute), nil))

		authorizationRequestDto, err := fixture.connectorAuthorizationApplication.GetConnectorAuthorizationRequest(
			t.Context(), "request-1")

		require.NoError(t, err)
		assert.Equal(t, dto.ConnectorAuthorizationRequestDto{
			ClientName: "Claude Code", ExpiresAt: connectorMoment.Add(time.Minute),
		}, authorizationRequestDto)
	})

	for _, testCase := range []struct {
		name                 string
		authorizationRequest entities.ConnectorAuthorizationRequest
	}{
		{name: "exactly ten minutes old", authorizationRequest: aPendingAuthorizationRequest(connectorMoment.Add(-10*time.Minute), nil)},
		{name: "already decided", authorizationRequest: aPendingAuthorizationRequest(connectorMoment.Add(-time.Minute), &decidedAt)},
	} {
		t.Run(testCase.name+" is not found", func(t *testing.T) {
			fixture := newConnectorAuthorizationApplicationUnderTest(t)
			fixture.expectAuthorizationRequest(testCase.authorizationRequest)

			_, err := fixture.connectorAuthorizationApplication.GetConnectorAuthorizationRequest(t.Context(), "request-1")

			require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
		})
	}

	t.Run("an unknown request is not found", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.connectorAuthorizationRequestRepository.EXPECT().
			FindOneByRequestIdentifier(gomock.Any(), "request-1").
			Return(entities.ConnectorAuthorizationRequest{}, domains.ErrConnectorAuthorizationRequestNotFound)

		_, err := fixture.connectorAuthorizationApplication.GetConnectorAuthorizationRequest(t.Context(), "request-1")

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("a request whose connector is gone is not found", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients()
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))

		_, err := fixture.connectorAuthorizationApplication.GetConnectorAuthorizationRequest(t.Context(), "request-1")

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("a storage failure looking up the connector is passed on", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		storageFailure := errors.New("connection closed")
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))
		fixture.connectorClientRepository.EXPECT().FindOneByClientIdentifier(gomock.Any(), "client-A").
			Return(entities.ConnectorClient{}, storageFailure)

		_, err := fixture.connectorAuthorizationApplication.GetConnectorAuthorizationRequest(t.Context(), "request-1")

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestConnectorAuthorizationApplicationApproveConnectorAuthorization(t *testing.T) {
	t.Run("approval issues a code bound to the user and sends it back with the state", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment.Add(-time.Minute), nil))
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "the-code", Digest: "the-code-digest"}, nil)
		fixture.connectorAuthorizationRequestRepository.EXPECT().
			Approve(gomock.Any(), uint(21), connectorMoment, entities.ConnectorAuthorizationCode{
				CodeDigest:                "the-code-digest",
				UserID:                    7,
				ConnectorClientIdentifier: "client-A",
				RedirectUri:               "http://localhost:51000/callback",
				CodeChallenge:             connectorCodeChallenge,
				Resource:                  connectorResource,
				ExpiresAt:                 connectorMoment.Add(5 * time.Minute),
				CreatedAt:                 connectorMoment,
			}).Return(nil)

		redirect, err := fixture.connectorAuthorizationApplication.ApproveConnectorAuthorization(t.Context(), "request-1", 7)

		require.NoError(t, err)
		assert.Equal(t, "http://localhost:51000/callback?code=the-code&state=abc", redirect.RedirectTo)
	})

	t.Run("losing a concurrent decision is not found", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "the-code"}, nil)
		fixture.connectorAuthorizationRequestRepository.EXPECT().Approve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(domains.ErrConnectorAuthorizationRequestNotFound)

		_, err := fixture.connectorAuthorizationApplication.ApproveConnectorAuthorization(t.Context(), "request-1", 7)

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("an expired request cannot be approved", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment.Add(-10*time.Minute), nil))

		_, err := fixture.connectorAuthorizationApplication.ApproveConnectorAuthorization(t.Context(), "request-1", 7)

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("a mint failure stores nothing", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		mintFailure := errors.New("no randomness")
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{}, mintFailure)

		_, err := fixture.connectorAuthorizationApplication.ApproveConnectorAuthorization(t.Context(), "request-1", 7)

		require.ErrorIs(t, err, mintFailure)
	})

	t.Run("a stored address that is no longer trusted is never redirected to", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		authorizationRequest := aPendingAuthorizationRequest(connectorMoment, nil)
		authorizationRequest.RedirectUri = "https://evil.example.com/cb"
		fixture.expectAuthorizationRequest(authorizationRequest)
		fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "the-code"}, nil)

		_, err := fixture.connectorAuthorizationApplication.ApproveConnectorAuthorization(t.Context(), "request-1", 7)

		require.ErrorIs(t, err, domains.ErrConnectorRedirectUriInvalid)
	})
}

func TestConnectorAuthorizationApplicationDenyConnectorAuthorization(t *testing.T) {
	t.Run("denial sends the browser back with access_denied and the state", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))
		fixture.connectorAuthorizationRequestRepository.EXPECT().Deny(gomock.Any(), uint(21), connectorMoment).Return(nil)

		redirect, err := fixture.connectorAuthorizationApplication.DenyConnectorAuthorization(t.Context(), "request-1")

		require.NoError(t, err)
		assert.Equal(t, "http://localhost:51000/callback?error=access_denied&state=abc", redirect.RedirectTo)
	})

	t.Run("a decided request cannot be denied", func(t *testing.T) {
		decidedAt := connectorMoment.Add(-time.Minute)
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, &decidedAt))

		_, err := fixture.connectorAuthorizationApplication.DenyConnectorAuthorization(t.Context(), "request-1")

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("losing a concurrent decision is not found", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectAuthorizationRequest(aPendingAuthorizationRequest(connectorMoment, nil))
		fixture.connectorAuthorizationRequestRepository.EXPECT().Deny(gomock.Any(), uint(21), connectorMoment).
			Return(domains.ErrConnectorAuthorizationRequestNotFound)

		_, err := fixture.connectorAuthorizationApplication.DenyConnectorAuthorization(t.Context(), "request-1")

		require.ErrorIs(t, err, domains.ErrConnectorAuthorizationRequestNotFound)
	})

	t.Run("an untrusted stored address is never redirected to", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		authorizationRequest := aPendingAuthorizationRequest(connectorMoment, nil)
		authorizationRequest.RedirectUri = "https://evil.example.com/cb"
		fixture.expectAuthorizationRequest(authorizationRequest)

		_, err := fixture.connectorAuthorizationApplication.DenyConnectorAuthorization(t.Context(), "request-1")

		require.ErrorIs(t, err, domains.ErrConnectorRedirectUriInvalid)
	})
}

func anAuthorizationCode(issuedAt time.Time) entities.ConnectorAuthorizationCode {
	return entities.ConnectorAuthorizationCode{
		ID: 31, CodeDigest: "the-code-digest", UserID: 7, ConnectorClientIdentifier: "client-A",
		RedirectUri: "http://localhost:51000/callback", CodeChallenge: connectorCodeChallenge,
		Resource: connectorResource, ExpiresAt: issuedAt.Add(5 * time.Minute), CreatedAt: issuedAt,
	}
}

func anExchangeDto() dto.ConnectorAuthorizationCodeExchangeDto {
	return dto.ConnectorAuthorizationCodeExchangeDto{
		Code: "the-code", RedirectUri: "http://localhost:51000/callback",
		ClientIdentifier: "client-A", CodeVerifier: connectorCodeVerifier,
	}
}

func (fixture connectorAuthorizationApplicationUnderTest) expectAuthorizationCode(
	authorizationCode entities.ConnectorAuthorizationCode,
) {
	fixture.connectorAuthorizationCodeRepository.EXPECT().
		FindOneByDigest(gomock.Any(), "the-code-digest").Return(authorizationCode, nil)
}

func TestConnectorAuthorizationApplicationExchangeAuthorizationCode(t *testing.T) {
	t.Run("a valid code opens a new chain for the connector with the audience on the token", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment.Add(-4 * time.Minute)))
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, Audience: connectorResource, ExpiresAt: accessTokenExpiryAfterConnectorMoment()}).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: accessTokenExpiryAfterConnectorMoment()}, nil)
		fixture.connectorAuthorizationCodeRepository.EXPECT().
			Redeem(gomock.Any(), uint(31), entities.Session{
				UserID:                    7,
				ChainID:                   "a-refresh-token-digest",
				RefreshTokenDigest:        "a-refresh-token-digest",
				ExpiresAt:                 connectorMoment.Add(30 * 24 * time.Hour),
				ConnectorClientIdentifier: "client-A",
				Audience:                  connectorResource,
			}).
			DoAndReturn(func(_ context.Context, _ uint, session entities.Session) (entities.Session, error) {
				return session, nil
			})

		tokens, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.NoError(t, err)
		assert.Equal(t, dto.ConnectorTokensDto{
			AccessToken: "a-signed-token", TokenType: "Bearer", ExpiresInSeconds: 900, RefreshToken: "a-refresh-token",
		}, tokens)
	})

	for _, testCase := range []struct {
		name   string
		code   entities.ConnectorAuthorizationCode
		change func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto)
	}{
		{name: "a code exactly five minutes old", code: anAuthorizationCode(connectorMoment.Add(-5 * time.Minute)), change: func(*dto.ConnectorAuthorizationCodeExchangeDto) {}},
		{name: "a wrong verifier", code: anAuthorizationCode(connectorMoment), change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("w", 43)
		}},
		{name: "a different port", code: anAuthorizationCode(connectorMoment), change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.RedirectUri = "http://localhost:33418/callback"
		}},
		{name: "another connector", code: anAuthorizationCode(connectorMoment), change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.ClientIdentifier = "client-B"
		}},
	} {
		t.Run(testCase.name+" is an invalid grant and opens nothing", func(t *testing.T) {
			fixture := newConnectorAuthorizationApplicationUnderTest(t)
			fixture.expectConnectorClients("client-A", "client-B")
			fixture.expectAuthorizationCode(testCase.code)
			exchangeDto := anExchangeDto()
			testCase.change(&exchangeDto)

			_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), exchangeDto)

			require.ErrorIs(t, err, domains.ErrConnectorGrantInvalid)
		})
	}

	t.Run("a replayed code tears down the chain it produced", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		redeemedCode := anAuthorizationCode(connectorMoment.Add(-2 * time.Minute))
		redeemedCode.SessionChainID = "chain-1"
		fixture.expectAuthorizationCode(redeemedCode)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "chain-1").Return(nil)

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, domains.ErrConnectorGrantInvalid)
	})

	t.Run("failing to tear down a replayed code's chain fails the request", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		storageFailure := errors.New("connection closed")
		redeemedCode := anAuthorizationCode(connectorMoment)
		redeemedCode.SessionChainID = "chain-1"
		fixture.expectAuthorizationCode(redeemedCode)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "chain-1").Return(storageFailure)

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, storageFailure)
	})

	t.Run("losing a concurrent exchange tears down the winner's chain", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().Issue(gomock.Any()).Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)
		fixture.connectorAuthorizationCodeRepository.EXPECT().Redeem(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrConnectorAuthorizationCodeAlreadyRedeemed)
		wonCode := anAuthorizationCode(connectorMoment)
		wonCode.SessionChainID = "winner-chain"
		fixture.expectAuthorizationCode(wonCode)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "winner-chain").Return(nil)

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, domains.ErrConnectorGrantInvalid)
	})

	t.Run("an unknown code is an invalid grant", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), "the-code-digest").
			Return(entities.ConnectorAuthorizationCode{}, domains.ErrConnectorAuthorizationCodeNotFound)

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, domains.ErrConnectorGrantInvalid)
	})

	t.Run("a code whose user is gone is an invalid grant", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients("client-A")
		fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{}, domains.ErrUserNotFound)

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, domains.ErrConnectorGrantInvalid)
	})

	t.Run("an unknown connector is an invalid client", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.expectConnectorClients()

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

		require.ErrorIs(t, err, domains.ErrConnectorClientNotFound)
	})

	t.Run("a request missing a field is invalid", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		exchangeDto := anExchangeDto()
		exchangeDto.CodeVerifier = ""

		_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), exchangeDto)

		require.ErrorIs(t, err, domains.ErrConnectorTokenRequestInvalid)
	})

	storageFailure := errors.New("connection closed")
	for _, testCase := range []struct {
		name    string
		arrange func(fixture connectorAuthorizationApplicationUnderTest)
	}{
		{name: "reading the code", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
				Return(entities.ConnectorAuthorizationCode{}, storageFailure)
		}},
		{name: "reading the user", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{}, storageFailure)
		}},
		{name: "minting", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
			fixture.refreshTokenProxy.EXPECT().Mint().Return(vo.RefreshTokenVo{}, storageFailure)
		}},
		{name: "signing", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
			fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
			fixture.accessTokenProxy.EXPECT().Issue(gomock.Any()).Return(vo.AccessTokenVo{}, storageFailure)
		}},
		{name: "redeeming", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
			fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
			fixture.accessTokenProxy.EXPECT().Issue(gomock.Any()).Return(vo.AccessTokenVo{}, nil)
			fixture.connectorAuthorizationCodeRepository.EXPECT().Redeem(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.Session{}, storageFailure)
		}},
		{name: "rereading a code lost in a race", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.expectAuthorizationCode(anAuthorizationCode(connectorMoment))
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
			fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
			fixture.accessTokenProxy.EXPECT().Issue(gomock.Any()).Return(vo.AccessTokenVo{}, nil)
			fixture.connectorAuthorizationCodeRepository.EXPECT().Redeem(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.Session{}, domains.ErrConnectorAuthorizationCodeAlreadyRedeemed)
			fixture.connectorAuthorizationCodeRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
				Return(entities.ConnectorAuthorizationCode{}, storageFailure)
		}},
	} {
		t.Run("a failure "+testCase.name+" is passed on", func(t *testing.T) {
			fixture := newConnectorAuthorizationApplicationUnderTest(t)
			fixture.expectConnectorClients("client-A")
			testCase.arrange(fixture)

			_, err := fixture.connectorAuthorizationApplication.ExchangeAuthorizationCode(t.Context(), anExchangeDto())

			require.ErrorIs(t, err, storageFailure)
		})
	}
}

func accessTokenExpiryAfterConnectorMoment() time.Time {
	return time.Date(2026, 9, 27, 8, 15, 0, 0, time.UTC)
}

func aConnectorSession(clientIdentifier string, revokedAt *time.Time) entities.Session {
	return entities.Session{
		ID: 41, UserID: 7, ChainID: "chain-1", RefreshTokenDigest: "a-connector-refresh-token-digest",
		ExpiresAt: connectorMoment.Add(time.Hour), RevokedAt: revokedAt,
		ConnectorClientIdentifier: clientIdentifier, Audience: connectorResource,
	}
}

func TestConnectorAuthorizationApplicationRenewConnectorSession(t *testing.T) {
	t.Run("renewal keeps the connector and audience", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), "a-connector-refresh-token-digest").
			Return(aConnectorSession("client-A", nil), nil)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, Audience: connectorResource, ExpiresAt: accessTokenExpiryAfterConnectorMoment()}).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: accessTokenExpiryAfterConnectorMoment()}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), uint(41), entities.Session{
				UserID: 7, ChainID: "chain-1", RefreshTokenDigest: "a-refresh-token-digest",
				ExpiresAt:                 connectorMoment.Add(30 * 24 * time.Hour),
				ConnectorClientIdentifier: "client-A", Audience: connectorResource,
			}).
			DoAndReturn(func(_ context.Context, _ uint, session entities.Session) (entities.Session, error) {
				return session, nil
			})

		tokens, err := fixture.connectorAuthorizationApplication.RenewConnectorSession(t.Context(), dto.SessionRenewalDto{
			RefreshToken: "a-connector-refresh-token", ConnectorClientIdentifier: "client-A",
		})

		require.NoError(t, err)
		assert.Equal(t, dto.ConnectorTokensDto{
			AccessToken: "a-signed-token", TokenType: "Bearer", ExpiresInSeconds: 900, RefreshToken: "a-refresh-token",
		}, tokens)
	})

	t.Run("a reused renewal token tears down the chain", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		revokedAt := connectorMoment.Add(-time.Minute)
		fixture.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aConnectorSession("client-A", &revokedAt), nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "chain-1").Return(nil)

		_, err := fixture.connectorAuthorizationApplication.RenewConnectorSession(t.Context(), dto.SessionRenewalDto{
			RefreshToken: "a-connector-refresh-token", ConnectorClientIdentifier: "client-A",
		})

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("another connector cannot renew it and nothing rotates", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aConnectorSession("client-A", nil), nil)

		_, err := fixture.connectorAuthorizationApplication.RenewConnectorSession(t.Context(), dto.SessionRenewalDto{
			RefreshToken: "a-connector-refresh-token", ConnectorClientIdentifier: "client-B",
		})

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("the web renewal path cannot renew a connector's session", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.sessionRepository.EXPECT().FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aConnectorSession("client-A", nil), nil)

		_, err := fixture.userApplication.RenewSession(t.Context(), dto.SessionRenewalDto{
			RefreshToken: "a-connector-refresh-token", ConnectorClientIdentifier: "client-A",
		})

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	for _, testCase := range []struct {
		name       string
		renewalDto dto.SessionRenewalDto
	}{
		{name: "no renewal token", renewalDto: dto.SessionRenewalDto{ConnectorClientIdentifier: "client-A"}},
		{name: "no connector", renewalDto: dto.SessionRenewalDto{RefreshToken: "a-connector-refresh-token"}},
	} {
		t.Run(testCase.name+" is an invalid request", func(t *testing.T) {
			fixture := newConnectorAuthorizationApplicationUnderTest(t)

			_, err := fixture.connectorAuthorizationApplication.RenewConnectorSession(t.Context(), testCase.renewalDto)

			require.ErrorIs(t, err, domains.ErrConnectorTokenRequestInvalid)
		})
	}
}

func TestConnectorAuthorizationApplicationIntrospectAccessToken(t *testing.T) {
	t.Run("a connector token reports its user, audience and expiry", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{
			UserID: 7, Audience: connectorResource, ExpiresAt: accessTokenExpiryAfterConnectorMoment(),
		}, nil)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)

		introspection, err := fixture.connectorAuthorizationApplication.IntrospectAccessToken(t.Context(), "a-token")

		require.NoError(t, err)
		subject, audience, expiresAt := "7", connectorResource, accessTokenExpiryAfterConnectorMoment().Unix()
		assert.Equal(t, dto.AccessTokenIntrospectionDto{
			Active: true, Subject: &subject, Audience: &audience, ExpiresAt: &expiresAt,
		}, introspection)
	})

	t.Run("a web token reports an empty audience", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		fixture.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{
			UserID: 7, ExpiresAt: accessTokenExpiryAfterConnectorMoment(),
		}, nil)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStoredUser(7, "james@example.com"), nil)

		introspection, err := fixture.connectorAuthorizationApplication.IntrospectAccessToken(t.Context(), "a-token")

		require.NoError(t, err)
		require.NotNil(t, introspection.Audience)
		assert.Equal(t, "", *introspection.Audience)
	})

	for _, testCase := range []struct {
		name        string
		accessToken string
		arrange     func(fixture connectorAuthorizationApplicationUnderTest)
	}{
		{name: "an empty token", accessToken: "", arrange: func(connectorAuthorizationApplicationUnderTest) {}},
		{name: "a broken token", accessToken: "a-token", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired)
		}},
		{name: "a token whose user is gone", accessToken: "a-token", arrange: func(fixture connectorAuthorizationApplicationUnderTest) {
			fixture.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{UserID: 7}, nil)
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{}, domains.ErrUserNotFound)
		}},
	} {
		t.Run(testCase.name+" is only inactive", func(t *testing.T) {
			fixture := newConnectorAuthorizationApplicationUnderTest(t)
			testCase.arrange(fixture)

			introspection, err := fixture.connectorAuthorizationApplication.IntrospectAccessToken(t.Context(), testCase.accessToken)

			require.NoError(t, err)
			assert.Equal(t, dto.AccessTokenIntrospectionDto{Active: false}, introspection)
		})
	}

	t.Run("a storage failure is not reported as inactive", func(t *testing.T) {
		fixture := newConnectorAuthorizationApplicationUnderTest(t)
		storageFailure := errors.New("connection closed")
		fixture.accessTokenProxy.EXPECT().ClaimsOf("a-token").Return(vo.AccessTokenClaimsVo{UserID: 7}, nil)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.User{}, storageFailure)

		_, err := fixture.connectorAuthorizationApplication.IntrospectAccessToken(t.Context(), "a-token")

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestConnectorAuthorizationApplicationNeverRedirectsIntoTheFrontEndWithAnUnescapedIdentifier(t *testing.T) {
	fixture := newConnectorAuthorizationApplicationUnderTest(t)
	fixture.expectConnectorClients("client-A")
	fixture.opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "a b&c"}, nil)
	fixture.connectorAuthorizationRequestRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.ConnectorAuthorizationRequest{}, nil)

	redirect, err := fixture.connectorAuthorizationApplication.StartConnectorAuthorization(t.Context(), aStartDto())

	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(redirect.RedirectTo, "?request=a+b%26c"))
}
