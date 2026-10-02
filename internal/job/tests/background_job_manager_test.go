package job_test

import (
	"context"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
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

func TestWaitAllReturnsOnceEveryJobHasFinished(t *testing.T) {
	mockController := gomock.NewController(t)
	finished := make(chan struct{})
	close(finished)
	backgroundJob := mocks.NewMockIBackgroundJob(mockController)
	backgroundJob.EXPECT().Finished().Return(finished)

	assert.True(t, job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{backgroundJob}).WaitAll(t.Context()))
}

func TestWaitAllGivesUpWhenAJobNeverFinishes(t *testing.T) {
	mockController := gomock.NewController(t)
	backgroundJob := mocks.NewMockIBackgroundJob(mockController)
	backgroundJob.EXPECT().Finished().Return(make(chan struct{}))
	waiting, giveUp := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer giveUp()

	assert.False(t, job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{backgroundJob}).WaitAll(waiting))
}

func TestAJobFinishesOnceStoppedAndItsRoundHasEnded(t *testing.T) {
	scanJob := newStrategyBotScanJobUnderTest(t, make(chan struct{}, 8), nil)
	scanJob.Start(t.Context())

	scanJob.Stop()

	select {
	case <-scanJob.Finished():
	case <-time.After(2 * time.Second):
		t.Fatal("a stopped job never said it had finished")
	}
}
