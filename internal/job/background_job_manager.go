package job

import domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"

// BackgroundJobManager starts every background job the system was assembled with.
// It knows none of them by name, so adding a job is a change to the assembly and
// not to this type.
type BackgroundJobManager struct {
	backgroundJobs []domaininterface.IBackgroundJob
}

func NewBackgroundJobManager(backgroundJobs []domaininterface.IBackgroundJob) *BackgroundJobManager {
	return &BackgroundJobManager{backgroundJobs: backgroundJobs}
}

// StartAll starts every job it holds. Holding none is the ordinary way to have
// background work switched off, and does nothing at all.
func (backgroundJobManager *BackgroundJobManager) StartAll() {
	for _, backgroundJob := range backgroundJobManager.backgroundJobs {
		backgroundJob.Start()
	}
}
