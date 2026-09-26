package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

type ConnectorClientDomain struct {
	connectorClient entities.ConnectorClient
}

func NewConnectorClientDomain(connectorClient entities.ConnectorClient) ConnectorClientDomain {
	return ConnectorClientDomain{connectorClient: connectorClient}
}

// RegisteredRedirectUri returns the address as sent, once it is known to belong to this connector.
func (connectorClientDomain ConnectorClientDomain) RegisteredRedirectUri(
	rawAddress string,
) (ConnectorRedirectUriDomain, error) {
	requested, requestedError := NewConnectorRedirectUriDomain(rawAddress)
	if requestedError != nil {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriNotRegistered
	}

	for _, registeredAddress := range connectorClientDomain.connectorClient.RedirectUris {
		registered, registeredError := NewConnectorRedirectUriDomain(registeredAddress)
		if registeredError == nil && registered.MatchesIgnoringPort(requested) {
			return requested, nil
		}
	}

	return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriNotRegistered
}
