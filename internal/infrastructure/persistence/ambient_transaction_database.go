package persistence

import (
	"context"

	"gorm.io/gorm"
)

// transactionContextKey carries the transaction a TransactionRepository opened down to the repositories that take part in it.
type transactionContextKey struct{}

// ambientTransactionDatabase lets a repository join whatever transaction its caller opened, and work on its own otherwise.
// A repository that should take part in ITransactionRepository.Atomically must read its database through this.
type ambientTransactionDatabase struct {
	root *gorm.DB
}

func (ambientTransactionDatabase ambientTransactionDatabase) within(executionContext context.Context) *gorm.DB {
	transaction, isInsideTransaction := executionContext.Value(transactionContextKey{}).(*gorm.DB)
	if isInsideTransaction {
		return transaction.WithContext(executionContext)
	}

	return ambientTransactionDatabase.root.WithContext(executionContext)
}
