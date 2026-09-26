package domains

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

type ConnectorAuthorizationCodeDomain struct {
	authorizationCode entities.ConnectorAuthorizationCode
}

func NewConnectorAuthorizationCodeDomain(
	authorizationCode entities.ConnectorAuthorizationCode,
) ConnectorAuthorizationCodeDomain {
	return ConnectorAuthorizationCodeDomain{authorizationCode: authorizationCode}
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) ID() uint {
	return connectorAuthorizationCodeDomain.authorizationCode.ID
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) UserID() uint {
	return connectorAuthorizationCodeDomain.authorizationCode.UserID
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) Audience() string {
	return connectorAuthorizationCodeDomain.authorizationCode.Resource
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) Redeemed() bool {
	return connectorAuthorizationCodeDomain.authorizationCode.SessionChainID != ""
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) SessionChainID() string {
	return connectorAuthorizationCodeDomain.authorizationCode.SessionChainID
}

func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) Expired(now time.Time) bool {
	return !now.Before(connectorAuthorizationCodeDomain.authorizationCode.ExpiresAt)
}

// Accepts compares the redirect address exactly (no port leniency here) and proves possession via S256.
func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) Accepts(
	exchange ConnectorAuthorizationCodeExchangeDomain,
) bool {
	authorizationCode := connectorAuthorizationCodeDomain.authorizationCode
	verifierDigest := sha256.Sum256([]byte(exchange.CodeVerifier()))
	derivedChallenge := base64.RawURLEncoding.EncodeToString(verifierDigest[:])

	return authorizationCode.ConnectorClientIdentifier == exchange.ClientIdentifier() &&
		authorizationCode.RedirectUri == exchange.RedirectUri() &&
		subtle.ConstantTimeCompare([]byte(derivedChallenge), []byte(authorizationCode.CodeChallenge)) == 1
}

// ToSession opens a new chain identified by the renewal-token digest, remembering the connector and audience.
func (connectorAuthorizationCodeDomain ConnectorAuthorizationCodeDomain) ToSession(
	refreshTokenDigest string, now time.Time, lifetime time.Duration,
) entities.Session {
	authorizationCode := connectorAuthorizationCodeDomain.authorizationCode

	return entities.Session{
		UserID:                    authorizationCode.UserID,
		ChainID:                   refreshTokenDigest,
		RefreshTokenDigest:        refreshTokenDigest,
		ExpiresAt:                 now.Add(lifetime).UTC(),
		ConnectorClientIdentifier: authorizationCode.ConnectorClientIdentifier,
		Audience:                  authorizationCode.Resource,
	}
}
