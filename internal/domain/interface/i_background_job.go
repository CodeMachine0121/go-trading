package _interface

import "context"

//go:generate go tool mockgen -source=i_background_job.go -destination=mocks/mock_i_background_job.go -package=mocks

// IBackgroundJob must not block in Start; Stop means finish in-hand work, while a done context abandons in-flight calls.
type IBackgroundJob interface {
	Start(executionContext context.Context)
	Stop()
	// Finished closes once the job has stopped, by Stop or by its context, and its in-flight round has ended.
	Finished() <-chan struct{}
}
