package domains

import (
	"net/url"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

type ConnectorAuthorizationRequestDomain struct {
	authorizationRequest entities.ConnectorAuthorizationRequest
}

func NewConnectorAuthorizationRequestDomain(
	authorizationRequest entities.ConnectorAuthorizationRequest,
) ConnectorAuthorizationRequestDomain {
	return ConnectorAuthorizationRequestDomain{authorizationRequest: authorizationRequest}
}

// Open treats the expiry instant itself as past, and a decided request as gone.
func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) Open(now time.Time) bool {
	return connectorAuthorizationRequestDomain.authorizationRequest.DecidedAt == nil &&
		now.Before(connectorAuthorizationRequestDomain.authorizationRequest.ExpiresAt)
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) ID() uint {
	return connectorAuthorizationRequestDomain.authorizationRequest.ID
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) ConnectorClientIdentifier() string {
	return connectorAuthorizationRequestDomain.authorizationRequest.ConnectorClientIdentifier
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) ToDto(
	clientName string,
) dto.ConnectorAuthorizationRequestDto {
	return dto.ConnectorAuthorizationRequestDto{
		ClientName: clientName,
		ExpiresAt:  connectorAuthorizationRequestDomain.authorizationRequest.ExpiresAt.UTC(),
	}
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) ToAuthorizationCode(
	userID uint, codeDigest string, now time.Time, lifetime time.Duration,
) entities.ConnectorAuthorizationCode {
	authorizationRequest := connectorAuthorizationRequestDomain.authorizationRequest

	return entities.ConnectorAuthorizationCode{
		CodeDigest:                codeDigest,
		UserID:                    userID,
		ConnectorClientIdentifier: authorizationRequest.ConnectorClientIdentifier,
		RedirectUri:               authorizationRequest.RedirectUri,
		CodeChallenge:             authorizationRequest.CodeChallenge,
		Resource:                  authorizationRequest.Resource,
		ExpiresAt:                 now.Add(lifetime).UTC(),
		CreatedAt:                 now.UTC(),
	}
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) ApprovalRedirect(
	code string,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	return connectorAuthorizationRequestDomain.redirectWith(url.Values{"code": {code}})
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) DenialRedirect() (
	dto.ConnectorAuthorizationRedirectDto, error,
) {
	return connectorAuthorizationRequestDomain.redirectWith(url.Values{"error": {"access_denied"}})
}

func (connectorAuthorizationRequestDomain ConnectorAuthorizationRequestDomain) redirectWith(
	parameters url.Values,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	authorizationRequest := connectorAuthorizationRequestDomain.authorizationRequest

	redirectUri, redirectUriError := NewConnectorRedirectUriDomain(authorizationRequest.RedirectUri)
	if redirectUriError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, redirectUriError
	}
	if authorizationRequest.State != "" {
		parameters.Set("state", authorizationRequest.State)
	}

	return dto.ConnectorAuthorizationRedirectDto{RedirectTo: redirectUri.WithParameters(parameters)}, nil
}
