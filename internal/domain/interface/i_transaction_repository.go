package _interface

import "context"

//go:generate go tool mockgen -source=i_transaction_repository.go -destination=mocks/mock_i_transaction_repository.go -package=mocks

// ITransactionRepository makes writes to several repositories land together or not at all; the work receives a context that carries the transaction, and only repositories reading it take part.
type ITransactionRepository interface {
	// Atomically runs work in one transaction, rolled back when work returns an error; inside another, it joins that one.
	Atomically(executionContext context.Context, work func(transactionContext context.Context) error) error
}
