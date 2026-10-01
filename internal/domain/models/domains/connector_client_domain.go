package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

type ConnectorClientDomain struct {
	connectorClient     entities.ConnectorClient
	trustedRedirectUris []string
}

func NewConnectorClientDomain(
	connectorClient entities.ConnectorClient, trustedRedirectUris []string,
) ConnectorClientDomain {
	return ConnectorClientDomain{connectorClient: connectorClient, trustedRedirectUris: trustedRedirectUris}
}

// RegisteredRedirectUri returns the address as sent, once it is known to belong to this connector.
func (connectorClientDomain ConnectorClientDomain) RegisteredRedirectUri(
	rawAddress string,
) (ConnectorRedirectUriDomain, error) {
	requested, requestedError := NewConnectorRedirectUriDomain(rawAddress, connectorClientDomain.trustedRedirectUris)
	if requestedError != nil {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriNotRegistered
	}

	for _, registeredAddress := range connectorClientDomain.connectorClient.RedirectUris {
		registered, registeredError := NewConnectorRedirectUriDomain(
			registeredAddress, connectorClientDomain.trustedRedirectUris)
		if registeredError == nil && registered.Matches(requested) {
			return requested, nil
		}
	}

	return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriNotRegistered
}
