package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func listeningOnAnyFreePort() *http.Server {
	return &http.Server{
		Addr:              "127.0.0.1:0",
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

func TestServeStopsTheJobsAndReturnsWhenShutdownIsSignalled(t *testing.T) {
	mockController := gomock.NewController(t)
	backgroundJob := mocks.NewMockIBackgroundJob(mockController)
	backgroundJob.EXPECT().Start(gomock.Any()).Times(1)
	backgroundJob.EXPECT().Stop().Times(1)

	// Follows hold requests open, so they must end before draining or shutdown waits out the grace period.
	liveFollowsStopped := make(chan struct{}, 1)

	shutdownSignalled, signalShutdown := context.WithCancel(t.Context())
	serveFinished := make(chan error, 1)
	go func() {
		serveFinished <- serve(
			shutdownSignalled,
			listeningOnAnyFreePort(),
			job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{backgroundJob}),
			func() { liveFollowsStopped <- struct{}{} },
		)
	}()

	signalShutdown()

	select {
	case serveError := <-serveFinished:
		assert.NoError(t, serveError)
		assert.Len(t, liveFollowsStopped, 1, "the live follows were not ended")
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown was signalled")
	}
}

func TestServeReportsAnAddressItCannotListenOn(t *testing.T) {
	mockController := gomock.NewController(t)
	backgroundJob := mocks.NewMockIBackgroundJob(mockController)
	backgroundJob.EXPECT().Start(gomock.Any()).Times(1)
	backgroundJob.EXPECT().Stop().Times(1)

	takenListener, listenError := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, listenError)
	t.Cleanup(func() { _ = takenListener.Close() })

	serveError := serve(t.Context(), &http.Server{
		Addr:              takenListener.Addr().String(),
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: readHeaderTimeout,
	}, job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{backgroundJob}), func() {})

	require.Error(t, serveError)
	assert.Contains(t, serveError.Error(), takenListener.Addr().String())
}

func TestServeWithNoBackgroundJobsStillShutsDown(t *testing.T) {
	shutdownSignalled, signalShutdown := context.WithCancel(t.Context())
	serveFinished := make(chan error, 1)
	go func() {
		serveFinished <- serve(
			shutdownSignalled,
			listeningOnAnyFreePort(),
			job.NewBackgroundJobManager([]domaininterface.IBackgroundJob{}),
			func() {},
		)
	}()

	signalShutdown()

	select {
	case serveError := <-serveFinished:
		assert.NoError(t, serveError)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown was signalled")
	}
}
