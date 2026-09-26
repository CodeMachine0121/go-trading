package domains

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

const (
	connectorRedirectUriCountLimit  = 10
	connectorClientNameLengthLimit  = 128
	unnamedConnectorClientName      = "未命名外掛"
	connectorTokenEndpointAuthNone  = "none"
	connectorAuthorizationCodeGrant = "authorization_code"
	connectorRefreshTokenGrant      = "refresh_token"
	connectorCodeResponseType       = "code"
)

type ConnectorClientRegistrationDomain struct {
	redirectUris []string
	clientName   string
}

func NewConnectorClientRegistrationDomain(
	registrationDto dto.ConnectorClientRegistrationDto,
) (ConnectorClientRegistrationDomain, error) {
	if len(registrationDto.RedirectUris) == 0 ||
		len(registrationDto.RedirectUris) > connectorRedirectUriCountLimit {
		return ConnectorClientRegistrationDomain{}, ErrConnectorRedirectUriInvalid
	}
	for _, redirectUri := range registrationDto.RedirectUris {
		if _, redirectUriError := NewConnectorRedirectUriDomain(redirectUri); redirectUriError != nil {
			return ConnectorClientRegistrationDomain{}, redirectUriError
		}
	}

	// A connector cannot keep a secret, so any secret-based method is refused rather than silently downgraded.
	if registrationDto.TokenEndpointAuthMethod != "" &&
		registrationDto.TokenEndpointAuthMethod != connectorTokenEndpointAuthNone {
		return ConnectorClientRegistrationDomain{}, ErrConnectorClientMetadataInvalid
	}
	for _, grantType := range registrationDto.GrantTypes {
		if grantType != connectorAuthorizationCodeGrant && grantType != connectorRefreshTokenGrant {
			return ConnectorClientRegistrationDomain{}, ErrConnectorClientMetadataInvalid
		}
	}
	for _, responseType := range registrationDto.ResponseTypes {
		if responseType != connectorCodeResponseType {
			return ConnectorClientRegistrationDomain{}, ErrConnectorClientMetadataInvalid
		}
	}

	clientName := strings.TrimSpace(registrationDto.ClientName)
	if utf8.RuneCountInString(clientName) > connectorClientNameLengthLimit {
		return ConnectorClientRegistrationDomain{}, ErrConnectorClientMetadataInvalid
	}
	if clientName == "" {
		clientName = unnamedConnectorClientName
	}

	return ConnectorClientRegistrationDomain{
		redirectUris: slices.Clone(registrationDto.RedirectUris),
		clientName:   clientName,
	}, nil
}

func (connectorClientRegistrationDomain ConnectorClientRegistrationDomain) ToEntity(
	clientIdentifier string, now time.Time,
) entities.ConnectorClient {
	return entities.ConnectorClient{
		ClientIdentifier: clientIdentifier,
		ClientName:       connectorClientRegistrationDomain.clientName,
		RedirectUris:     connectorClientRegistrationDomain.redirectUris,
		CreatedAt:        now.UTC(),
	}
}
