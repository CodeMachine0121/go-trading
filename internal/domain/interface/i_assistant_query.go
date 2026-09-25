package _interface

import "context"

//go:generate go tool mockgen -source=i_assistant_query.go -destination=mocks/mock_i_assistant_query.go -package=mocks

// IAssistantQuery is one capability the assistant is given; it can do nothing without an implementation, and a refusal from the underlying use case is returned to the assistant as a reason rather than a failure.
type IAssistantQuery interface {
	// Name must be unique among the capabilities offered.
	Name() string
	Description() string
	// ArgumentSchema is schema text handed to the assistant untouched, so the domain keeps it as an opaque string.
	ArgumentSchema() string
	// Run returns what the assistant should read; an error is a refusal reason the assistant may act on, not the end of the answer.
	// The viewer is an explicit parameter so forgetting it is a compile error; market-only capabilities ignore it.
	Run(executionContext context.Context, viewerID uint, arguments string) (string, error)
}
