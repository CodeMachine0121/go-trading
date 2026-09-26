package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type ConnectorAuthorizationCodeExchangeDomain struct {
	exchangeDto dto.ConnectorAuthorizationCodeExchangeDto
}

func NewConnectorAuthorizationCodeExchangeDomain(
	exchangeDto dto.ConnectorAuthorizationCodeExchangeDto,
) (ConnectorAuthorizationCodeExchangeDomain, error) {
	if exchangeDto.Code == "" || exchangeDto.RedirectUri == "" ||
		exchangeDto.ClientIdentifier == "" || exchangeDto.CodeVerifier == "" {
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
