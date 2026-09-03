package _interface

import "context"

//go:generate go tool mockgen -source=i_background_job.go -destination=mocks/mock_i_background_job.go -package=mocks

// IBackgroundJob is work the system does on its own, with nobody asking. Starting
// it must not block: a job that needs to keep running does so on its own.
//
// The two ways it ends say different things. Stop means stop taking on new work and
// let whatever is in hand finish, which is what an orderly shutdown asks for first.
// The context being done means the time for finishing has run out, and reaches the
// work itself — a call already out to the database or the market source is abandoned
// rather than waited for.
type IBackgroundJob interface {
	Start(executionContext context.Context)
	Stop()
}
