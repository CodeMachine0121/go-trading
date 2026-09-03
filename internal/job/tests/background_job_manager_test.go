package job_test

import (
	"testing"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"go.uber.org/mock/gomock"
)

func TestStartAllStartsEveryJobItHolds(t *testing.T) {
	mockController := gomock.NewController(t)
	backgroundJobs := make([]domaininterface.IBackgroundJob, 0, 3)
	for range 3 {
		backgroundJob := mocks.NewMockIBackgroundJob(mockController)
		backgroundJob.EXPECT().Start(gomock.Any()).Times(1)
		backgroundJobs = append(backgroundJobs, backgroundJob)
	}

	job.NewBackgroundJobManager(backgroundJobs).StartAll(t.Context())
}

func TestStartAllWithNoJobsDoesNothing(t *testing.T) {
	job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{}).StartAll(t.Context())
}

func TestStopAllStopsEveryJobItHolds(t *testing.T) {
	mockController := gomock.NewController(t)
	backgroundJobs := make([]domaininterface.IBackgroundJob, 0, 3)
	for range 3 {
		backgroundJob := mocks.NewMockIBackgroundJob(mockController)
		backgroundJob.EXPECT().Stop().Times(1)
		backgroundJobs = append(backgroundJobs, backgroundJob)
	}

	job.NewBackgroundJobManager(backgroundJobs).StopAll()
}

func TestStopAllWithNoJobsDoesNothing(t *testing.T) {
	job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{}).StopAll()
}
