package job

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
)

// BackgroundJobManager starts every background job the system was assembled with.
// It knows none of them by name, so adding a job is a change to the assembly and
// not to this type.
type BackgroundJobManager struct {
	backgroundJobs []domaininterface.IBackgroundJob
}

func NewBackgroundJobManager(backgroundJobs []domaininterface.IBackgroundJob) *BackgroundJobManager {
	return &BackgroundJobManager{backgroundJobs: backgroundJobs}
}

// StartAll starts every job it holds, each under the same context. Holding none is
// the ordinary way to have background work switched off, and does nothing at all.
func (backgroundJobManager *BackgroundJobManager) StartAll(executionContext context.Context) {
	for _, backgroundJob := range backgroundJobManager.backgroundJobs {
		backgroundJob.Start(executionContext)
	}
}

// StopAll stops every job it holds, in the order they were given. Stopping asks each
// job to take on no further work; it does not wait for the work in hand, because the
// waiting belongs to whoever owns the context the jobs were started under and can
// therefore decide how long is long enough.
func (backgroundJobManager *BackgroundJobManager) StopAll() {
	for _, backgroundJob := range backgroundJobManager.backgroundJobs {
		backgroundJob.Stop()
	}
}
