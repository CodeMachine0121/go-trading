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

// listeningOnAnyFreePort is the server serve is given when the test does not care
// which port it lands on, which is every test here: nothing connects to it.
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

	// A viewer following a market holds its request open for as long as it is fed,
	// so the follows have to end before the requests are drained. Were it the other
	// way round, every shutdown would sit out the whole grace period.
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

// A port already in use is the ordinary way starting up fails, and it must be
// reported rather than sat on: a server that never listened has to say so, because
// nothing else will notice that nobody is being served.
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

// Holding no jobs is how background work is switched off, and shutting down must
// still work when there is nothing to stop.
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
