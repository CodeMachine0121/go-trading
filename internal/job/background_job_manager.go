package job

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
)

type BackgroundJobManager struct {
	backgroundJobs []domaininterface.IBackgroundJob
}

func NewBackgroundJobManager(backgroundJobs []domaininterface.IBackgroundJob) *BackgroundJobManager {
	return &BackgroundJobManager{backgroundJobs: backgroundJobs}
}

// StartAll with no jobs is how background work is switched off.
func (backgroundJobManager *BackgroundJobManager) StartAll(executionContext context.Context) {
	for _, backgroundJob := range backgroundJobManager.backgroundJobs {
		backgroundJob.Start(executionContext)
	}
}

// StopAll stops jobs in order without waiting for in-flight work; the context owner decides how long to wait.
func (backgroundJobManager *BackgroundJobManager) StopAll() {
	for _, backgroundJob := range backgroundJobManager.backgroundJobs {
		backgroundJob.Stop()
	}
}
