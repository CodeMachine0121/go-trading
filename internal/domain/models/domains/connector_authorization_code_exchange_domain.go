package domains

import (
	"regexp"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// connectorCodeVerifierShape is RFC 7636 §4.1: 43 to 128 unreserved characters.
var connectorCodeVerifierShape = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type ConnectorAuthorizationCodeExchangeDomain struct {
	exchangeDto dto.ConnectorAuthorizationCodeExchangeDto
}

func NewConnectorAuthorizationCodeExchangeDomain(
	exchangeDto dto.ConnectorAuthorizationCodeExchangeDto,
) (ConnectorAuthorizationCodeExchangeDomain, error) {
	if exchangeDto.Code == "" || exchangeDto.RedirectUri == "" ||
		exchangeDto.ClientIdentifier == "" || !connectorCodeVerifierShape.MatchString(exchangeDto.CodeVerifier) {
		return ConnectorAuthorizationCodeExchangeDomain{}, ErrConnectorTokenRequestInvalid
	}

	return ConnectorAuthorizationCodeExchangeDomain{exchangeDto: exchangeDto}, nil
}

func (connectorAuthorizationCodeExchangeDomain ConnectorAuthorizationCodeExchangeDomain) Code() string {
	return connectorAuthorizationCodeExchangeDomain.exchangeDto.Code
}

func (connectorAuthorizationCodeExchangeDomain ConnectorAuthorizationCodeExchangeDomain) ClientIdentifier() string {
	return connectorAuthorizationCodeExchangeDomain.exchangeDto.ClientIdentifier
}

func (connectorAuthorizationCodeExchangeDomain ConnectorAuthorizationCodeExchangeDomain) RedirectUri() string {
	return connectorAuthorizationCodeExchangeDomain.exchangeDto.RedirectUri
}

func (connectorAuthorizationCodeExchangeDomain ConnectorAuthorizationCodeExchangeDomain) CodeVerifier() string {
	return connectorAuthorizationCodeExchangeDomain.exchangeDto.CodeVerifier
}
