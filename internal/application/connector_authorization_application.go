package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ConnectorAuthorizationApplication hands connector renewal to the user service, since a connector's sign-in is an ordinary session chain.
type ConnectorAuthorizationApplication struct {
	connectorAuthorizationService *service.ConnectorAuthorizationService
	userService                   *service.UserService
}

func NewConnectorAuthorizationApplication(
	connectorAuthorizationService *service.ConnectorAuthorizationService,
	userService *service.UserService,
) *ConnectorAuthorizationApplication {
	return &ConnectorAuthorizationApplication{
		connectorAuthorizationService: connectorAuthorizationService,
		userService:                   userService,
	}
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) DescribeAuthorizationServer() dto.ConnectorAuthorizationServerMetadataDto {
	return connectorAuthorizationApplication.connectorAuthorizationService.DescribeAuthorizationServer()
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) RegisterConnectorClient(
	executionContext context.Context, registrationDto dto.ConnectorClientRegistrationDto,
) (dto.ConnectorClientDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.RegisterConnectorClient(
		executionContext, registrationDto)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) StartConnectorAuthorization(
	executionContext context.Context, startDto dto.ConnectorAuthorizationStartDto,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.StartConnectorAuthorization(
		executionContext, startDto)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) GetConnectorAuthorizationRequest(
	executionContext context.Context, requestIdentifier string,
) (dto.ConnectorAuthorizationRequestDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.GetConnectorAuthorizationRequest(
		executionContext, requestIdentifier)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) ApproveConnectorAuthorization(
	executionContext context.Context, requestIdentifier string, userID uint,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.ApproveConnectorAuthorization(
		executionContext, requestIdentifier, userID)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) DenyConnectorAuthorization(
	executionContext context.Context, requestIdentifier string,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.DenyConnectorAuthorization(
		executionContext, requestIdentifier)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) ExchangeAuthorizationCode(
	executionContext context.Context, exchangeDto dto.ConnectorAuthorizationCodeExchangeDto,
) (dto.ConnectorTokensDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.ExchangeAuthorizationCode(
		executionContext, exchangeDto)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) RenewConnectorSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) (dto.ConnectorTokensDto, error) {
	return connectorAuthorizationApplication.userService.RenewConnectorSession(executionContext, renewalDto)
}

func (connectorAuthorizationApplication *ConnectorAuthorizationApplication) IntrospectAccessToken(
	executionContext context.Context, accessToken string,
) (dto.AccessTokenIntrospectionDto, error) {
	return connectorAuthorizationApplication.connectorAuthorizationService.IntrospectAccessToken(
		executionContext, accessToken)
}
