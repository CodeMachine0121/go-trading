package domains

import (
	"net/url"
	"strings"
)

const connectorRedirectUriLengthLimit = 2048

var loopbackHostnames = []string{"localhost", "127.0.0.1", "::1"}

// ConnectorRedirectUriDomain accepts only plain-http loopback addresses: anything reachable from elsewhere would let a stranger collect someone else's code.
type ConnectorRedirectUriDomain struct {
	address *url.URL
}

func NewConnectorRedirectUriDomain(rawAddress string) (ConnectorRedirectUriDomain, error) {
	if len(rawAddress) > connectorRedirectUriLengthLimit {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
	}

	address, parseError := url.Parse(rawAddress)
	if parseError != nil ||
		!strings.EqualFold(address.Scheme, "http") ||
		address.User != nil ||
		address.Fragment != "" ||
		strings.Contains(rawAddress, "#") {
		return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
	}

	for _, loopbackHostname := range loopbackHostnames {
		if strings.EqualFold(address.Hostname(), loopbackHostname) {
			return ConnectorRedirectUriDomain{address: address}, nil
		}
	}

	return ConnectorRedirectUriDomain{}, ErrConnectorRedirectUriInvalid
}

// MatchesIgnoringPort follows RFC 8252 §7.3: a loopback connector picks a fresh port on every launch.
func (connectorRedirectUriDomain ConnectorRedirectUriDomain) MatchesIgnoringPort(
	other ConnectorRedirectUriDomain,
) bool {
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
