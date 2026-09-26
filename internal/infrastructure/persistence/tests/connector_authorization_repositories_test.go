package persistence_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var decisionMoment = time.Date(2026, 9, 27, 8, 1, 2, 0, time.UTC)

func aStoredConnectorClient(t *testing.T, database *gorm.DB, clientIdentifier string) entities.ConnectorClient {
	t.Helper()

	connectorClient, saveError := persistence.NewConnectorClientRepository(database).Save(t.Context(), entities.ConnectorClient{
		ClientIdentifier: clientIdentifier, ClientName: "Claude Code",
		RedirectUris: []string{"http://localhost:33418/callback", "http://127.0.0.1/cb"},
		CreatedAt:    time.Now().UTC(),
	})
	require.NoError(t, saveError)

	return connectorClient
}

func aStoredAuthorizationRequest(t *testing.T, database *gorm.DB, requestIdentifier string) entities.ConnectorAuthorizationRequest {
	t.Helper()

	authorizationRequest, saveError := persistence.NewConnectorAuthorizationRequestRepository(database).Save(
		t.Context(), entities.ConnectorAuthorizationRequest{
			RequestIdentifier: requestIdentifier, ConnectorClientIdentifier: "client-A",
			RedirectUri: "http://localhost:51000/callback", CodeChallenge: "a-challenge",
			State: "abc", Resource: "https://mcp.example.com/mcp",
			ExpiresAt: time.Now().Add(10 * time.Minute).UTC(), CreatedAt: time.Now().UTC(),
		})
	require.NoError(t, saveError)

	return authorizationRequest
}

func anAuthorizationCodeFor(userID uint, codeDigest string) entities.ConnectorAuthorizationCode {
	return entities.ConnectorAuthorizationCode{
		CodeDigest: codeDigest, UserID: userID, ConnectorClientIdentifier: "client-A",
		RedirectUri: "http://localhost:51000/callback", CodeChallenge: "a-challenge",
		Resource: "https://mcp.example.com/mcp", ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
		CreatedAt: time.Now().UTC(),
	}
}

func TestConnectorClientRepositoryFindsAConnectorByItsIdentifier(t *testing.T) {
	database := newTestDatabase(t)
	savedClient := aStoredConnectorClient(t, database, "client-A")
	connectorClientRepository := persistence.NewConnectorClientRepository(database)

	foundClient, findError := connectorClientRepository.FindOneByClientIdentifier(t.Context(), "client-A")

	require.NoError(t, findError)
	assert.Equal(t, savedClient.ID, foundClient.ID)
	assert.Equal(t, []string{"http://localhost:33418/callback", "http://127.0.0.1/cb"}, foundClient.RedirectUris)

	_, missingError := connectorClientRepository.FindOneByClientIdentifier(t.Context(), "nobody")
	assert.ErrorIs(t, missingError, domains.ErrConnectorClientNotFound)
}

func TestConnectorAuthorizationRequestRepositoryFindsARequestByItsIdentifier(t *testing.T) {
	database := newTestDatabase(t)
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)

	foundRequest, findError := requestRepository.FindOneByRequestIdentifier(t.Context(), "request-1")

	require.NoError(t, findError)
	assert.Equal(t, savedRequest.ID, foundRequest.ID)
	assert.Equal(t, "http://localhost:51000/callback", foundRequest.RedirectUri)
	assert.Nil(t, foundRequest.DecidedAt)

	_, missingError := requestRepository.FindOneByRequestIdentifier(t.Context(), "request-2")
	assert.ErrorIs(t, missingError, domains.ErrConnectorAuthorizationRequestNotFound)
}

func TestConnectorAuthorizationRequestRepositoryApprovesOnceAndStoresTheCode(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)

	approveError := requestRepository.Approve(t.Context(), savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1"))
	require.NoError(t, approveError)

	decidedRequest, findError := requestRepository.FindOneByRequestIdentifier(t.Context(), "request-1")
	require.NoError(t, findError)
	require.NotNil(t, decidedRequest.DecidedAt)
	assert.True(t, decisionMoment.Equal(*decidedRequest.DecidedAt), "決定時刻要是呼叫端給的時刻")
	storedCode, codeError := persistence.NewConnectorAuthorizationCodeRepository(database).FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, codeError)
	assert.Equal(t, owner.ID, storedCode.UserID)

	secondError := requestRepository.Approve(t.Context(), savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-2"))
	assert.ErrorIs(t, secondError, domains.ErrConnectorAuthorizationRequestNotFound)
	_, secondCodeError := persistence.NewConnectorAuthorizationCodeRepository(database).FindOneByDigest(t.Context(), "code-2")
	assert.ErrorIs(t, secondCodeError, domains.ErrConnectorAuthorizationCodeNotFound, "輸掉的那一次不能留下授權碼")
}

func TestConnectorAuthorizationRequestRepositoryLetsOnlyOneOfConcurrentDecisionsThrough(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)

	results := make([]error, 2)
	waitGroup := sync.WaitGroup{}
	waitGroup.Go(func() {
		results[0] = requestRepository.Approve(t.Context(), savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1"))
	})
	waitGroup.Go(func() {
		results[1] = requestRepository.Deny(t.Context(), savedRequest.ID, decisionMoment)
	})
	waitGroup.Wait()

	succeeded := 0
	for _, result := range results {
		if result == nil {
			succeeded++
			continue
		}
		assert.ErrorIs(t, result, domains.ErrConnectorAuthorizationRequestNotFound)
	}
	assert.Equal(t, 1, succeeded)
}

func TestConnectorAuthorizationRequestRepositoryDeniesOnce(t *testing.T) {
	database := newTestDatabase(t)
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)

	require.NoError(t, requestRepository.Deny(t.Context(), savedRequest.ID, decisionMoment))
	deniedRequest, findError := requestRepository.FindOneByRequestIdentifier(t.Context(), "request-1")
	require.NoError(t, findError)
	require.NotNil(t, deniedRequest.DecidedAt)
	assert.True(t, decisionMoment.Equal(*deniedRequest.DecidedAt), "決定時刻要是呼叫端給的時刻")

	assert.ErrorIs(t, requestRepository.Deny(t.Context(), savedRequest.ID, decisionMoment), domains.ErrConnectorAuthorizationRequestNotFound)
}

func TestConnectorAuthorizationCodeRepositoryRedeemsOnceAndRemembersTheChain(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	require.NoError(t, persistence.NewConnectorAuthorizationRequestRepository(database).Approve(t.Context(), savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1")))
	codeRepository := persistence.NewConnectorAuthorizationCodeRepository(database)
	storedCode, findError := codeRepository.FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, findError)
	connectorSession := sessionOf(owner.ID, "chain-1", "digest-1")
	connectorSession.ConnectorClientIdentifier = "client-A"
	connectorSession.Audience = "https://mcp.example.com/mcp"

	savedSession, redeemError := codeRepository.Redeem(t.Context(), storedCode.ID, connectorSession)

	require.NoError(t, redeemError)
	assert.Positive(t, savedSession.ID)
	redeemedCode, rereadError := codeRepository.FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, rereadError)
	assert.Equal(t, "chain-1", redeemedCode.SessionChainID)
	storedSession, sessionError := persistence.NewSessionRepository(database).FindOneByDigest(t.Context(), "digest-1")
	require.NoError(t, sessionError)
	assert.Equal(t, "client-A", storedSession.ConnectorClientIdentifier)
	assert.Equal(t, "https://mcp.example.com/mcp", storedSession.Audience)

	_, secondError := codeRepository.Redeem(t.Context(), storedCode.ID, sessionOf(owner.ID, "chain-2", "digest-2"))
	assert.ErrorIs(t, secondError, domains.ErrConnectorAuthorizationCodeAlreadyRedeemed)
	_, secondSessionError := persistence.NewSessionRepository(database).FindOneByDigest(t.Context(), "digest-2")
	assert.ErrorIs(t, secondSessionError, domains.ErrSessionNotFound, "輸掉的那一次不能留下登入階段")
}

func TestConnectorAuthorizationCodeRepositoryFindsNothingForAnUnknownDigest(t *testing.T) {
	database := newTestDatabase(t)

	_, findError := persistence.NewConnectorAuthorizationCodeRepository(database).FindOneByDigest(t.Context(), "nothing")

	assert.ErrorIs(t, findError, domains.ErrConnectorAuthorizationCodeNotFound)
}

func TestConnectorAuthorizationRepositoriesReportStorageFailuresAsSuch(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	connectorClientRepository := persistence.NewConnectorClientRepository(database)
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)
	codeRepository := persistence.NewConnectorAuthorizationCodeRepository(database)
	cancelledContext, cancel := context.WithCancel(t.Context())
	cancel()

	testCases := []struct {
		name   string
		action func() error
	}{
		{name: "saving a connector", action: func() error {
			_, err := connectorClientRepository.Save(cancelledContext, entities.ConnectorClient{ClientIdentifier: "client-B"})
			return err
		}},
		{name: "finding a connector", action: func() error {
			_, err := connectorClientRepository.FindOneByClientIdentifier(cancelledContext, "client-A")
			return err
		}},
		{name: "saving a request", action: func() error {
			_, err := requestRepository.Save(cancelledContext, entities.ConnectorAuthorizationRequest{RequestIdentifier: "request-2"})
			return err
		}},
		{name: "finding a request", action: func() error {
			_, err := requestRepository.FindOneByRequestIdentifier(cancelledContext, "request-1")
			return err
		}},
		{name: "denying", action: func() error {
			return requestRepository.Deny(cancelledContext, savedRequest.ID, decisionMoment)
		}},
		{name: "approving", action: func() error {
			return requestRepository.Approve(cancelledContext, savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1"))
		}},
		{name: "finding a code", action: func() error {
			_, err := codeRepository.FindOneByDigest(cancelledContext, "code-1")
			return err
		}},
		{name: "redeeming", action: func() error {
			_, err := codeRepository.Redeem(cancelledContext, 1, sessionOf(owner.ID, "chain-1", "digest-1"))
			return err
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.action()

			require.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestConnectorAuthorizationRepositoriesUndoTheDecisionWhenTheSecondWriteFails(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	firstRequest := aStoredAuthorizationRequest(t, database, "request-1")
	secondRequest := aStoredAuthorizationRequest(t, database, "request-2")
	requestRepository := persistence.NewConnectorAuthorizationRequestRepository(database)
	codeRepository := persistence.NewConnectorAuthorizationCodeRepository(database)
	require.NoError(t, requestRepository.Approve(t.Context(), firstRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1")))

	duplicateCodeError := requestRepository.Approve(t.Context(), secondRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1"))

	require.Error(t, duplicateCodeError)
	stillOpen, findError := requestRepository.FindOneByRequestIdentifier(t.Context(), "request-2")
	require.NoError(t, findError)
	assert.Nil(t, stillOpen.DecidedAt, "授權碼沒寫進去，這筆請求就不算決定過")

	storedCode, codeError := codeRepository.FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, codeError)
	_, sessionSaveError := persistence.NewSessionRepository(database).Save(t.Context(), sessionOf(owner.ID, "web-chain", "taken-digest"))
	require.NoError(t, sessionSaveError)

	_, duplicateSessionError := codeRepository.Redeem(t.Context(), storedCode.ID, sessionOf(owner.ID, "chain-1", "taken-digest"))

	require.Error(t, duplicateSessionError)
	unredeemed, rereadError := codeRepository.FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, rereadError)
	assert.Empty(t, unredeemed.SessionChainID, "登入階段沒開成，授權碼就不算用過")
}

func TestConnectorAuthorizationCodeRepositoryReportsAChainItCannotRecord(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	aStoredConnectorClient(t, database, "client-A")
	savedRequest := aStoredAuthorizationRequest(t, database, "request-1")
	require.NoError(t, persistence.NewConnectorAuthorizationRequestRepository(database).Approve(t.Context(), savedRequest.ID, decisionMoment, anAuthorizationCodeFor(owner.ID, "code-1")))
	codeRepository := persistence.NewConnectorAuthorizationCodeRepository(database)
	storedCode, findError := codeRepository.FindOneByDigest(t.Context(), "code-1")
	require.NoError(t, findError)

	_, redeemError := codeRepository.Redeem(t.Context(), storedCode.ID,
		sessionOf(owner.ID, strings.Repeat("c", 65), "digest-1"))

	require.Error(t, redeemError)
	assert.NotErrorIs(t, redeemError, domains.ErrConnectorAuthorizationCodeAlreadyRedeemed)
}
