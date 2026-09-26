package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConnectorClientRepository struct {
	database *gorm.DB
}

func NewConnectorClientRepository(database *gorm.DB) *ConnectorClientRepository {
	return &ConnectorClientRepository{database: database}
}

func (connectorClientRepository *ConnectorClientRepository) Save(
	executionContext context.Context, connectorClient entities.ConnectorClient,
) (entities.ConnectorClient, error) {
	result := connectorClientRepository.database.WithContext(executionContext).Create(&connectorClient)
	if result.Error != nil {
		return entities.ConnectorClient{}, fmt.Errorf("save connector client: %w", result.Error)
	}

	return connectorClient, nil
}

func (connectorClientRepository *ConnectorClientRepository) FindOneByClientIdentifier(
	executionContext context.Context, clientIdentifier string,
) (entities.ConnectorClient, error) {
	connectorClient := entities.ConnectorClient{}

	result := connectorClientRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "client_identifier", Value: clientIdentifier}).
		First(&connectorClient)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ConnectorClient{}, domains.ErrConnectorClientNotFound
	}
	if result.Error != nil {
		return entities.ConnectorClient{}, fmt.Errorf("find connector client: %w", result.Error)
	}

	return connectorClient, nil
}
