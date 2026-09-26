package domains_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var connectorMoment = time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)

const (
	rfcCodeVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	rfcCodeChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

func TestConnectorClientRegistrationAcceptsOnlyLoopbackHttpAddresses(t *testing.T) {
	testCases := []struct {
		name          string
		redirectUris  []string
		expectedError error
	}{
		{name: "localhost with a port and path", redirectUris: []string{"http://localhost:33418/callback"}},
		{name: "IPv4 loopback", redirectUris: []string{"http://127.0.0.1:1/x"}},
		{name: "IPv6 loopback on any port", redirectUris: []string{"http://[::1]:8765/"}},
		{name: "a public site", redirectUris: []string{"https://evil.example.com/cb"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "loopback over https", redirectUris: []string{"https://localhost:33418/callback"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "a plain http public host", redirectUris: []string{"http://example.com/cb"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "no address at all", redirectUris: nil, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "a fragment", redirectUris: []string{"http://localhost:1/cb#frag"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "an empty fragment", redirectUris: []string{"http://localhost:1/cb#"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "credentials in the address", redirectUris: []string{"http://user@localhost/cb"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "an unparseable address", redirectUris: []string{"http://localhost:port/cb"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "an overlong address", redirectUris: []string{"http://localhost/" + strings.Repeat("a", 2048)}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{name: "one bad among good", redirectUris: []string{"http://localhost/cb", "https://evil.example.com"}, expectedError: domains.ErrConnectorRedirectUriInvalid},
		{
			name:          "eleven addresses",
			redirectUris:  []string{"http://localhost/1", "http://localhost/2", "http://localhost/3", "http://localhost/4", "http://localhost/5", "http://localhost/6", "http://localhost/7", "http://localhost/8", "http://localhost/9", "http://localhost/10", "http://localhost/11"},
			expectedError: domains.ErrConnectorRedirectUriInvalid,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewConnectorClientRegistrationDomain(
				dto.ConnectorClientRegistrationDto{RedirectUris: testCase.redirectUris})

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, testCase.expectedError)
		})
	}
}

func TestConnectorClientRegistrationRefusesMetadataANoSecretConnectorCannotUse(t *testing.T) {
	testCases := []struct {
		name            string
		registrationDto dto.ConnectorClientRegistrationDto
		expectedError   error
	}{
		{name: "a secret-based auth method", registrationDto: dto.ConnectorClientRegistrationDto{TokenEndpointAuthMethod: "client_secret_basic"}, expectedError: domains.ErrConnectorClientMetadataInvalid},
		{name: "auth method none given explicitly", registrationDto: dto.ConnectorClientRegistrationDto{TokenEndpointAuthMethod: "none"}},
		{name: "the two supported grants", registrationDto: dto.ConnectorClientRegistrationDto{GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}}},
		{name: "an unsupported grant", registrationDto: dto.ConnectorClientRegistrationDto{GrantTypes: []string{"client_credentials"}}, expectedError: domains.ErrConnectorClientMetadataInvalid},
		{name: "an unsupported response type", registrationDto: dto.ConnectorClientRegistrationDto{ResponseTypes: []string{"token"}}, expectedError: domains.ErrConnectorClientMetadataInvalid},
		{name: "a 128-character name", registrationDto: dto.ConnectorClientRegistrationDto{ClientName: strings.Repeat("名", 128)}},
		{name: "a 129-character name", registrationDto: dto.ConnectorClientRegistrationDto{ClientName: strings.Repeat("名", 129)}, expectedError: domains.ErrConnectorClientMetadataInvalid},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			registrationDto := testCase.registrationDto
			registrationDto.RedirectUris = []string{"http://localhost:33418/callback"}

			_, err := domains.NewConnectorClientRegistrationDomain(registrationDto)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, testCase.expectedError)
		})
	}
}

func TestConnectorClientRegistrationNamesAnUnnamedConnector(t *testing.T) {
	testCases := []struct {
		name         string
		clientName   string
		expectedName string
	}{
		{name: "blank", clientName: "   ", expectedName: "未命名外掛"},
		{name: "padded", clientName: "  Claude Code  ", expectedName: "Claude Code"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			registration, err := domains.NewConnectorClientRegistrationDomain(dto.ConnectorClientRegistrationDto{
				RedirectUris: []string{"http://localhost:33418/callback"},
				ClientName:   testCase.clientName,
			})
			require.NoError(t, err)

			connectorClient := registration.ToEntity("client-A", connectorMoment)

			assert.Equal(t, testCase.expectedName, connectorClient.ClientName)
			assert.Equal(t, "client-A", connectorClient.ClientIdentifier)
			assert.Equal(t, []string{"http://localhost:33418/callback"}, connectorClient.RedirectUris)
			assert.Equal(t, connectorMoment, connectorClient.CreatedAt)
		})
	}
}

func TestConnectorClientRecognisesItsAddressesIgnoringOnlyThePort(t *testing.T) {
	connectorClient := domains.NewConnectorClientDomain(entities.ConnectorClient{
		RedirectUris: []string{"http://localhost:33418/callback", "http://127.0.0.1/cb?mode=a"},
	})

	testCases := []struct {
		name       string
		requested  string
		registered bool
	}{
		{name: "exactly as registered", requested: "http://localhost:33418/callback", registered: true},
		{name: "another port", requested: "http://localhost:51000/callback", registered: true},
		{name: "no port", requested: "http://localhost/callback", registered: true},
		{name: "host in capitals", requested: "http://LOCALHOST:5/callback", registered: true},
		{name: "same query", requested: "http://127.0.0.1:9/cb?mode=a", registered: true},
		{name: "another path", requested: "http://localhost:33418/other", registered: false},
		{name: "another loopback host", requested: "http://127.0.0.1:33418/callback", registered: false},
		{name: "another query", requested: "http://127.0.0.1:9/cb?mode=b", registered: false},
		{name: "not loopback", requested: "http://example.com:33418/callback", registered: false},
		{name: "missing", requested: "", registered: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := connectorClient.RegisteredRedirectUri(testCase.requested)

			if testCase.registered {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, domains.ErrConnectorRedirectUriNotRegistered)
		})
	}
}

func TestConnectorAuthorizationStartSendsIncompleteRequestsBackToTheConnector(t *testing.T) {
	complete := dto.ConnectorAuthorizationStartDto{
		ResponseType: "code", ClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback",
		CodeChallenge: rfcCodeChallenge, CodeChallengeMethod: "S256", State: "abc",
	}

	testCases := []struct {
		name             string
		change           func(startDto *dto.ConnectorAuthorizationStartDto)
		refused          bool
		expectedRedirect string
	}{
		{name: "complete", change: func(*dto.ConnectorAuthorizationStartDto) {}},
		{
			name:    "no challenge",
			change:  func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.CodeChallenge = "" },
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name:    "plain challenge method",
			change:  func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.CodeChallengeMethod = "plain" },
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name:    "not asking for a code",
			change:  func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.ResponseType = "token" },
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name: "an overlong state",
			change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.State = strings.Repeat("s", 2049)
			},
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name: "an overlong challenge",
			change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.CodeChallenge = strings.Repeat("c", 129)
			},
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name:    "a challenge one character short",
			change:  func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.CodeChallenge = strings.Repeat("c", 42) },
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name:    "a challenge one character long",
			change:  func(startDto *dto.ConnectorAuthorizationStartDto) { startDto.CodeChallenge = strings.Repeat("c", 44) },
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name: "a challenge in padded standard base64",
			change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.CodeChallenge = strings.Repeat("c", 41) + "+="
			},
			refused: true, expectedRedirect: "error=invalid_request",
		},
		{
			name: "a base64url challenge of 43 characters",
			change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.CodeChallenge = strings.Repeat("A", 41) + "-_"
			},
		},
		{
			name: "an overlong resource",
			change: func(startDto *dto.ConnectorAuthorizationStartDto) {
				startDto.Resource = strings.Repeat("r", 2049)
			},
			refused: true, expectedRedirect: "error=invalid_request",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			startDto := complete
			testCase.change(&startDto)
			redirectUri, err := domains.NewConnectorRedirectUriDomain(startDto.RedirectUri)
			require.NoError(t, err)

			refusal, refused := domains.NewConnectorAuthorizationStartDomain(startDto, redirectUri).RefusalRedirect()

			assert.Equal(t, testCase.refused, refused)
			if testCase.refused {
				assert.True(t, strings.HasPrefix(refusal.RedirectTo, "http://localhost:51000/callback?"))
				assert.Contains(t, refusal.RedirectTo, testCase.expectedRedirect)
			}
		})
	}
}

func TestConnectorAuthorizationStartEchoesTheStateOnlyWhenGiven(t *testing.T) {
	testCases := []struct {
		name     string
		state    string
		expected string
	}{
		{name: "with state", state: "abc", expected: "state=abc"},
		{name: "without state", state: "", expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			redirectUri, err := domains.NewConnectorRedirectUriDomain("http://localhost:33418/callback")
			require.NoError(t, err)

			refusal, _ := domains.NewConnectorAuthorizationStartDomain(
				dto.ConnectorAuthorizationStartDto{ResponseType: "code", State: testCase.state}, redirectUri,
			).RefusalRedirect()

			if testCase.expected == "" {
				assert.NotContains(t, refusal.RedirectTo, "state=")
				return
			}
			assert.Contains(t, refusal.RedirectTo, testCase.expected)
		})
	}
}

func TestConnectorAuthorizationStartRecordsTheRequestAsSent(t *testing.T) {
	redirectUri, err := domains.NewConnectorRedirectUriDomain("http://localhost:51000/callback")
	require.NoError(t, err)

	authorizationRequest := domains.NewConnectorAuthorizationStartDomain(dto.ConnectorAuthorizationStartDto{
		ResponseType: "code", ClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback",
		CodeChallenge: rfcCodeChallenge, CodeChallengeMethod: "S256", State: "abc", Resource: "https://mcp.example.com/mcp",
	}, redirectUri).ToEntity("request-1", connectorMoment, 10*time.Minute)

	assert.Equal(t, entities.ConnectorAuthorizationRequest{
		RequestIdentifier:         "request-1",
		ConnectorClientIdentifier: "client-A",
		RedirectUri:               "http://localhost:51000/callback",
		CodeChallenge:             rfcCodeChallenge,
		State:                     "abc",
		Resource:                  "https://mcp.example.com/mcp",
		ExpiresAt:                 connectorMoment.Add(10 * time.Minute),
		CreatedAt:                 connectorMoment,
	}, authorizationRequest)
}

func TestConnectorAuthorizationRequestIsOpenUntilItExpiresOrIsDecided(t *testing.T) {
	decidedAt := connectorMoment.Add(-time.Minute)

	testCases := []struct {
		name      string
		createdAt time.Time
		decidedAt *time.Time
		open      bool
	}{
		{name: "nine minutes old", createdAt: connectorMoment.Add(-9 * time.Minute), open: true},
		{name: "exactly ten minutes old", createdAt: connectorMoment.Add(-10 * time.Minute), open: false},
		{name: "already decided", createdAt: connectorMoment.Add(-2 * time.Minute), decidedAt: &decidedAt, open: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			authorizationRequest := domains.NewConnectorAuthorizationRequestDomain(entities.ConnectorAuthorizationRequest{
				ExpiresAt: testCase.createdAt.Add(10 * time.Minute),
				DecidedAt: testCase.decidedAt,
			})

			assert.Equal(t, testCase.open, authorizationRequest.Open(connectorMoment))
		})
	}
}

func TestConnectorAuthorizationRequestSendsTheBrowserBackWithTheDecision(t *testing.T) {
	testCases := []struct {
		name     string
		state    string
		decide   func(authorizationRequest domains.ConnectorAuthorizationRequestDomain) (dto.ConnectorAuthorizationRedirectDto, error)
		expected string
	}{
		{
			name:  "approved",
			state: "abc",
			decide: func(authorizationRequest domains.ConnectorAuthorizationRequestDomain) (dto.ConnectorAuthorizationRedirectDto, error) {
				return authorizationRequest.ApprovalRedirect("the-code")
			},
			expected: "http://localhost:51000/callback?code=the-code&state=abc",
		},
		{
			name:  "denied",
			state: "abc",
			decide: func(authorizationRequest domains.ConnectorAuthorizationRequestDomain) (dto.ConnectorAuthorizationRedirectDto, error) {
				return authorizationRequest.DenialRedirect()
			},
			expected: "http://localhost:51000/callback?error=access_denied&state=abc",
		},
		{
			name: "denied without state",
			decide: func(authorizationRequest domains.ConnectorAuthorizationRequestDomain) (dto.ConnectorAuthorizationRedirectDto, error) {
				return authorizationRequest.DenialRedirect()
			},
			expected: "http://localhost:51000/callback?error=access_denied",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			redirect, err := testCase.decide(domains.NewConnectorAuthorizationRequestDomain(entities.ConnectorAuthorizationRequest{
				RedirectUri: "http://localhost:51000/callback", State: testCase.state,
			}))

			require.NoError(t, err)
			assert.Equal(t, testCase.expected, redirect.RedirectTo)
		})
	}
}

func TestConnectorAuthorizationRequestRefusesToRedirectToAStoredUntrustedAddress(t *testing.T) {
	_, err := domains.NewConnectorAuthorizationRequestDomain(entities.ConnectorAuthorizationRequest{
		RedirectUri: "https://evil.example.com/cb",
	}).DenialRedirect()

	assert.ErrorIs(t, err, domains.ErrConnectorRedirectUriInvalid)
}

func TestConnectorAuthorizationRequestBindsTheCodeToEverythingItWasAskedWith(t *testing.T) {
	authorizationCode := domains.NewConnectorAuthorizationRequestDomain(entities.ConnectorAuthorizationRequest{
		ConnectorClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback",
		CodeChallenge: rfcCodeChallenge, Resource: "https://mcp.example.com/mcp",
	}).ToAuthorizationCode(7, "code-digest", connectorMoment, 5*time.Minute)

	assert.Equal(t, entities.ConnectorAuthorizationCode{
		CodeDigest:                "code-digest",
		UserID:                    7,
		ConnectorClientIdentifier: "client-A",
		RedirectUri:               "http://localhost:51000/callback",
		CodeChallenge:             rfcCodeChallenge,
		Resource:                  "https://mcp.example.com/mcp",
		ExpiresAt:                 connectorMoment.Add(5 * time.Minute),
		CreatedAt:                 connectorMoment,
	}, authorizationCode)
}

func TestConnectorAuthorizationCodeExchangeNeedsEveryField(t *testing.T) {
	complete := dto.ConnectorAuthorizationCodeExchangeDto{
		Code: "the-code", RedirectUri: "http://localhost:51000/callback", ClientIdentifier: "client-A", CodeVerifier: rfcCodeVerifier,
	}

	testCases := []struct {
		name   string
		change func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto)
		valid  bool
	}{
		{name: "complete", change: func(*dto.ConnectorAuthorizationCodeExchangeDto) {}, valid: true},
		{name: "no code", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) { exchangeDto.Code = "" }},
		{name: "no verifier", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) { exchangeDto.CodeVerifier = "" }},
		{name: "no redirect address", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) { exchangeDto.RedirectUri = "" }},
		{name: "no client", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) { exchangeDto.ClientIdentifier = "" }},
		{name: "a verifier of 42 characters", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("v", 42)
		}},
		{name: "a verifier of 43 characters", valid: true, change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("v", 43)
		}},
		{name: "a verifier of 128 characters", valid: true, change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("v", 128)
		}},
		{name: "a verifier of 129 characters", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("v", 129)
		}},
		{name: "a verifier using every unreserved character", valid: true, change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = "AZaz09-._~" + strings.Repeat("v", 33)
		}},
		{name: "a verifier with a reserved character", change: func(exchangeDto *dto.ConnectorAuthorizationCodeExchangeDto) {
			exchangeDto.CodeVerifier = strings.Repeat("v", 42) + "+"
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exchangeDto := complete
			testCase.change(&exchangeDto)

			_, err := domains.NewConnectorAuthorizationCodeExchangeDomain(exchangeDto)

			if testCase.valid {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, domains.ErrConnectorTokenRequestInvalid)
		})
	}
}

func TestConnectorAuthorizationCodeAcceptsOnlyItsOwnConnectorAddressAndVerifier(t *testing.T) {
	authorizationCode := domains.NewConnectorAuthorizationCodeDomain(entities.ConnectorAuthorizationCode{
		ConnectorClientIdentifier: "client-A", RedirectUri: "http://localhost:51000/callback", CodeChallenge: rfcCodeChallenge,
	})

	testCases := []struct {
		name             string
		clientIdentifier string
		redirectUri      string
		codeVerifier     string
		accepted         bool
	}{
		{name: "the right verifier", clientIdentifier: "client-A", redirectUri: "http://localhost:51000/callback", codeVerifier: rfcCodeVerifier, accepted: true},
		{name: "a wrong verifier", clientIdentifier: "client-A", redirectUri: "http://localhost:51000/callback", codeVerifier: strings.Repeat("w", 43)},
		{name: "a different port", clientIdentifier: "client-A", redirectUri: "http://localhost:33418/callback", codeVerifier: rfcCodeVerifier},
		{name: "another connector", clientIdentifier: "client-B", redirectUri: "http://localhost:51000/callback", codeVerifier: rfcCodeVerifier},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exchange, err := domains.NewConnectorAuthorizationCodeExchangeDomain(dto.ConnectorAuthorizationCodeExchangeDto{
				Code: "the-code", ClientIdentifier: testCase.clientIdentifier, RedirectUri: testCase.redirectUri, CodeVerifier: testCase.codeVerifier,
			})
			require.NoError(t, err)

			assert.Equal(t, testCase.accepted, authorizationCode.Accepts(exchange))
		})
	}
}

func TestConnectorAuthorizationCodeExpiresAfterFiveMinutes(t *testing.T) {
	testCases := []struct {
		name     string
		issuedAt time.Time
		expired  bool
	}{
		{name: "four minutes old", issuedAt: connectorMoment.Add(-4 * time.Minute), expired: false},
		{name: "exactly five minutes old", issuedAt: connectorMoment.Add(-5 * time.Minute), expired: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			authorizationCode := domains.NewConnectorAuthorizationCodeDomain(entities.ConnectorAuthorizationCode{
				ExpiresAt: testCase.issuedAt.Add(5 * time.Minute),
			})

			assert.Equal(t, testCase.expired, authorizationCode.Expired(connectorMoment))
		})
	}
}

func TestConnectorAuthorizationCodeOpensAChainRememberingTheConnectorAndAudience(t *testing.T) {
	session := domains.NewConnectorAuthorizationCodeDomain(entities.ConnectorAuthorizationCode{
		UserID: 7, ConnectorClientIdentifier: "client-A", Resource: "https://mcp.example.com/mcp",
	}).ToSession("refresh-digest", connectorMoment, 30*24*time.Hour)

	assert.Equal(t, entities.Session{
		UserID:                    7,
		ChainID:                   "refresh-digest",
		RefreshTokenDigest:        "refresh-digest",
		ExpiresAt:                 connectorMoment.Add(30 * 24 * time.Hour),
		ConnectorClientIdentifier: "client-A",
		Audience:                  "https://mcp.example.com/mcp",
	}, session)
}

func TestSessionDomainRenewsOnlyThroughTheChannelThatOpenedIt(t *testing.T) {
	testCases := []struct {
		name                      string
		sessionClientIdentifier   string
		presentedClientIdentifier string
		held                      bool
	}{
		{name: "web session renewed on the web", held: true},
		{name: "connector session renewed by its connector", sessionClientIdentifier: "client-A", presentedClientIdentifier: "client-A", held: true},
		{name: "connector session renewed on the web", sessionClientIdentifier: "client-A", held: false},
		{name: "connector session renewed by another connector", sessionClientIdentifier: "client-A", presentedClientIdentifier: "client-B", held: false},
		{name: "web session renewed by a connector", presentedClientIdentifier: "client-A", held: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := domains.NewSessionDomain(entities.Session{ConnectorClientIdentifier: testCase.sessionClientIdentifier})

			assert.Equal(t, testCase.held, session.HeldBy(testCase.presentedClientIdentifier))
		})
	}
}

func TestSessionDomainCarriesTheConnectorAndAudienceIntoTheRenewedSession(t *testing.T) {
	renewed := domains.NewSessionDomain(entities.Session{
		UserID: 7, ChainID: "chain-1", ConnectorClientIdentifier: "client-A", Audience: "https://mcp.example.com/mcp",
	}).Renewed("next-digest", connectorMoment, time.Hour)

	assert.Equal(t, "client-A", renewed.ConnectorClientIdentifier)
	assert.Equal(t, "https://mcp.example.com/mcp", renewed.Audience)
}
