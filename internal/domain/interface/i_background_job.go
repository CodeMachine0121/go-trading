package _interface

import "context"

//go:generate go tool mockgen -source=i_background_job.go -destination=mocks/mock_i_background_job.go -package=mocks

// IBackgroundJob must not block in Start; Stop means finish in-hand work, while a done context abandons in-flight calls.
type IBackgroundJob interface {
	Start(executionContext context.Context)
	Stop()
}
