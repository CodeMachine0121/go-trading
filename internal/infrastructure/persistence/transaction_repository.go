package persistence

import (
	"context"

	"gorm.io/gorm"
)

type TransactionRepository struct {
	database *gorm.DB
}

func NewTransactionRepository(database *gorm.DB) *TransactionRepository {
	return &TransactionRepository{database: database}
}

// Atomically joins a transaction already in the context rather than nesting, so the outermost caller decides what commits.
func (transactionRepository *TransactionRepository) Atomically(
	executionContext context.Context, work func(transactionContext context.Context) error,
) error {
	if _, isInsideTransaction := executionContext.Value(transactionContextKey{}).(*gorm.DB); isInsideTransaction {
		return work(executionContext)
	}

	return transactionRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			return work(context.WithValue(executionContext, transactionContextKey{}, transaction))
		})
}
