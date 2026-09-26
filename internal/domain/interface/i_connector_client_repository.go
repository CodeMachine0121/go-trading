package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_connector_client_repository.go -destination=mocks/mock_i_connector_client_repository.go -package=mocks

type IConnectorClientRepository interface {
	Save(executionContext context.Context, connectorClient entities.ConnectorClient) (entities.ConnectorClient, error)
	// FindOneByClientIdentifier returns ErrConnectorClientNotFound when absent.
	FindOneByClientIdentifier(executionContext context.Context, clientIdentifier string) (entities.ConnectorClient, error)
}
