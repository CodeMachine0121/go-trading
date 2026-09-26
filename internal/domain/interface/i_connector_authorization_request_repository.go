package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_connector_authorization_request_repository.go -destination=mocks/mock_i_connector_authorization_request_repository.go -package=mocks

// IConnectorAuthorizationRequestRepository decides a request at write time, so two concurrent decisions cannot both succeed.
type IConnectorAuthorizationRequestRepository interface {
	Save(
		executionContext context.Context, authorizationRequest entities.ConnectorAuthorizationRequest,
	) (entities.ConnectorAuthorizationRequest, error)
	// FindOneByRequestIdentifier returns decided and expired requests too; ErrConnectorAuthorizationRequestNotFound when absent.
	FindOneByRequestIdentifier(
		executionContext context.Context, requestIdentifier string,
	) (entities.ConnectorAuthorizationRequest, error)
	// Approve marks the request decided and stores the code in one transaction, only if still undecided; otherwise ErrConnectorAuthorizationRequestNotFound.
	Approve(
		executionContext context.Context, requestID uint, authorizationCode entities.ConnectorAuthorizationCode,
	) error
	// Deny marks the request decided only if still undecided; otherwise ErrConnectorAuthorizationRequestNotFound.
	Deny(executionContext context.Context, requestID uint) error
}
