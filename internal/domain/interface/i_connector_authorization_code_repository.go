package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_connector_authorization_code_repository.go -destination=mocks/mock_i_connector_authorization_code_repository.go -package=mocks

type IConnectorAuthorizationCodeRepository interface {
	// FindOneByDigest returns redeemed codes too, because presenting one is the signal of a leaked code; ErrConnectorAuthorizationCodeNotFound when absent.
	FindOneByDigest(executionContext context.Context, codeDigest string) (entities.ConnectorAuthorizationCode, error)
	// Redeem records the new session's chain on the code and stores the session in one transaction, only if not yet redeemed; otherwise ErrConnectorAuthorizationCodeAlreadyRedeemed.
	Redeem(
		executionContext context.Context, codeID uint, session entities.Session,
	) (entities.Session, error)
}
