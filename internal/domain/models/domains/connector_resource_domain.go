package domains

import (
	"net/url"
	"slices"
	"strings"
)

// ConnectorResourceDomain is the service a connector's tokens are bound to; RFC 8707 §2 requires an absolute address without a fragment and advises against a query.
type ConnectorResourceDomain struct {
	address *url.URL
}

// NewConnectorResourceDomain allows plain http only on loopback, so a local connector server can still be developed against.
func NewConnectorResourceDomain(rawResource string) (ConnectorResourceDomain, error) {
	address, parseError := url.Parse(rawResource)
	if parseError != nil ||
		address.Hostname() == "" ||
		address.User != nil ||
		strings.ContainsAny(rawResource, "#?") {
		return ConnectorResourceDomain{}, ErrConnectorResourceInvalid
	}

	loopback := slices.ContainsFunc(loopbackHostnames, func(loopbackHostname string) bool {
		return strings.EqualFold(address.Hostname(), loopbackHostname)
	})
	if !strings.EqualFold(address.Scheme, "https") && !(loopback && strings.EqualFold(address.Scheme, "http")) {
		return ConnectorResourceDomain{}, ErrConnectorResourceInvalid
	}

	return ConnectorResourceDomain{address: address}, nil
}
