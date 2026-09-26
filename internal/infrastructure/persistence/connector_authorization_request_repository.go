package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConnectorAuthorizationRequestRepository struct {
	database *gorm.DB
}

func NewConnectorAuthorizationRequestRepository(database *gorm.DB) *ConnectorAuthorizationRequestRepository {
	return &ConnectorAuthorizationRequestRepository{database: database}
}

func (connectorAuthorizationRequestRepository *ConnectorAuthorizationRequestRepository) Save(
	executionContext context.Context, authorizationRequest entities.ConnectorAuthorizationRequest,
) (entities.ConnectorAuthorizationRequest, error) {
	result := connectorAuthorizationRequestRepository.database.WithContext(executionContext).
		Create(&authorizationRequest)
	if result.Error != nil {
		return entities.ConnectorAuthorizationRequest{}, fmt.Errorf(
			"save connector authorization request: %w", result.Error)
	}

	return authorizationRequest, nil
}

func (connectorAuthorizationRequestRepository *ConnectorAuthorizationRequestRepository) FindOneByRequestIdentifier(
	executionContext context.Context, requestIdentifier string,
) (entities.ConnectorAuthorizationRequest, error) {
	authorizationRequest := entities.ConnectorAuthorizationRequest{}

	result := connectorAuthorizationRequestRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "request_identifier", Value: requestIdentifier}).
		First(&authorizationRequest)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ConnectorAuthorizationRequest{}, domains.ErrConnectorAuthorizationRequestNotFound
	}
	if result.Error != nil {
		return entities.ConnectorAuthorizationRequest{}, fmt.Errorf(
			"find connector authorization request: %w", result.Error)
	}

	return authorizationRequest, nil
}

func (connectorAuthorizationRequestRepository *ConnectorAuthorizationRequestRepository) Approve(
	executionContext context.Context, requestID uint, decidedAt time.Time,
	authorizationCode entities.ConnectorAuthorizationCode,
) error {
	return connectorAuthorizationRequestRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			if decideError := connectorAuthorizationRequestRepository.decide(
				transaction, requestID, decidedAt); decideError != nil {
				return decideError
			}

			if created := transaction.Create(&authorizationCode); created.Error != nil {
				return fmt.Errorf("save connector authorization code: %w", created.Error)
			}

			return nil
		})
}

func (connectorAuthorizationRequestRepository *ConnectorAuthorizationRequestRepository) Deny(
	executionContext context.Context, requestID uint, decidedAt time.Time,
) error {
	return connectorAuthorizationRequestRepository.decide(
		connectorAuthorizationRequestRepository.database.WithContext(executionContext), requestID, decidedAt)
}

// decide puts the not-yet-decided condition on the update itself, so a concurrent decision makes it touch zero rows.
func (connectorAuthorizationRequestRepository *ConnectorAuthorizationRequestRepository) decide(
	database *gorm.DB, requestID uint, decidedAt time.Time,
) error {
	decided := database.
		Model(&entities.ConnectorAuthorizationRequest{}).
		Where(clause.Eq{Column: "id", Value: requestID}).
		Where(clause.Eq{Column: "decided_at", Value: nil}).
		Update("decided_at", decidedAt.UTC())
	if decided.Error != nil {
		return fmt.Errorf("decide connector authorization request: %w", decided.Error)
	}
	if decided.RowsAffected == 0 {
		return domains.ErrConnectorAuthorizationRequestNotFound
	}

	return nil
}
