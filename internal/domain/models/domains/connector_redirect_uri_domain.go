package domains

import (
	"net/url"
	"slices"
	"strings"
)

const connectorRedirectUriLengthLimit = 2048

var loopbackHostnames = []string{"localhost", "127.0.0.1", "::1"}

// ConnectorRedirectUriDomain accepts plain-http loopback addresses and, for hosted connectors such as Claude Desktop, only the exact https addresses the operator trusts: any other reachable address would let a stranger collect someone else's code.
type ConnectorRedirectUriDomain struct {
	address  *url.URL
	loopback bool
}

func NewConnectorRedirectUriDomain(
	rawAddress string, trustedRedirectUris []string,
) (ConnectorRedirectUriDomain, error) {
	if len(rawAddress) > connectorRedirectUriLengthLimit {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
	}

	address, parseError := url.Parse(rawAddress)
	if parseError != nil ||
		address.User != nil ||
		address.Fragment != "" ||
		strings.Contains(rawAddress, "#") {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
	}

	if strings.EqualFold(address.Scheme, "https") && slices.Contains(trustedRedirectUris, rawAddress) {
		return ConnectorRedirectUriDomain{address: address}, nil
	}

	if !strings.EqualFold(address.Scheme, "http") {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
	}
	for _, loopbackHostname := range loopbackHostnames {
		if strings.EqualFold(address.Hostname(), loopbackHostname) {
			return ConnectorRedirectUriDomain{address: address, loopback: true}, nil
		}
	}

	return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
}

// Matches ignores the port only between loopback addresses, per RFC 8252 §7.3: a loopback connector picks a fresh port on every launch.
func (connectorRedirectUriDomain ConnectorRedirectUriDomain) Matches(
	other ConnectorRedirectUriDomain,
) bool {
	if !connectorRedirectUriDomain.loopback || !other.loopback {
		return connectorRedirectUriDomain.address.String() == other.address.String()
	}

	return strings.EqualFold(connectorRedirectUriDomain.address.Scheme, other.address.Scheme) &&
		strings.EqualFold(connectorRedirectUriDomain.address.Hostname(), other.address.Hostname()) &&
		connectorRedirectUriDomain.address.EscapedPath() == other.address.EscapedPath() &&
		connectorRedirectUriDomain.address.RawQuery == other.address.RawQuery
}

// WithParameters keeps any query the connector registered and adds the given parameters to it.
func (connectorRedirectUriDomain ConnectorRedirectUriDomain) WithParameters(parameters url.Values) string {
	address := *connectorRedirectUriDomain.address
	query := address.Query()
	for name, values := range parameters {
		query[name] = values
	}
	address.RawQuery = query.Encode()

	return address.String()
}
