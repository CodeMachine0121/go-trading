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

type ConnectorAuthorizationCodeRepository struct {
	database *gorm.DB
}

func NewConnectorAuthorizationCodeRepository(database *gorm.DB) *ConnectorAuthorizationCodeRepository {
	return &ConnectorAuthorizationCodeRepository{database: database}
}

func (connectorAuthorizationCodeRepository *ConnectorAuthorizationCodeRepository) FindOneByDigest(
	executionContext context.Context, codeDigest string,
) (entities.ConnectorAuthorizationCode, error) {
	authorizationCode := entities.ConnectorAuthorizationCode{}

	result := connectorAuthorizationCodeRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "code_digest", Value: codeDigest}).
		First(&authorizationCode)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ConnectorAuthorizationCode{}, domains.ErrConnectorAuthorizationCodeNotFound
	}
	if result.Error != nil {
		return entities.ConnectorAuthorizationCode{}, fmt.Errorf(
			"find connector authorization code: %w", result.Error)
	}

	return authorizationCode, nil
}

func (connectorAuthorizationCodeRepository *ConnectorAuthorizationCodeRepository) Redeem(
	executionContext context.Context, codeID uint, session entities.Session,
) (entities.Session, error) {
	transactionError := connectorAuthorizationCodeRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			redeemed := transaction.
				Model(&entities.ConnectorAuthorizationCode{}).
				Where(clause.Eq{Column: "id", Value: codeID}).
				Where(clause.Eq{Column: "session_chain_id", Value: ""}).
				Update("session_chain_id", session.ChainID)
			if redeemed.Error != nil {
				return fmt.Errorf("redeem connector authorization code: %w", redeemed.Error)
			}
			if redeemed.RowsAffected == 0 {
				return domains.ErrConnectorAuthorizationCodeAlreadyRedeemed
			}

			if created := transaction.Create(&session); created.Error != nil {
				return fmt.Errorf("save connector session: %w", created.Error)
			}

			return nil
		})
	if transactionError != nil {
		return entities.Session{}, transactionError
	}

	return session, nil
}
