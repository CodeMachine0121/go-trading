package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
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

func TestServerKeepsAStreamOpenPastTheReadTimeoutButDropsASlowBody(t *testing.T) {
	testCases := []struct {
		name            string
		request         string
		expectedOutcome string
	}{
		{
			name:            "a live stream outlives the read timeout",
			request:         "GET /stream HTTP/1.1\r\nHost: test\r\n\r\n",
			expectedOutcome: "streamed past the read timeout",
		},
		{
			name:            "a body that never finishes arriving is abandoned",
			request:         "POST /upload HTTP/1.1\r\nHost: test\r\nContent-Length: 100\r\n\r\npartial",
			expectedOutcome: "gave up on the body",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("SERVER_READ_TIMEOUT_SECONDS", "1")
			outcomes := make(chan string, 1)
			handler := http.NewServeMux()
			handler.HandleFunc("/stream", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				for range 3 {
					select {
					case <-request.Context().Done():
						outcomes <- "cut off"
						return
					case <-time.After(time.Second):
						_, _ = writer.Write([]byte("data: {}\n\n"))
						http.NewResponseController(writer).Flush()
					}
				}
				outcomes <- "streamed past the read timeout"
			})
			handler.HandleFunc("/upload", func(writer http.ResponseWriter, request *http.Request) {
				if _, readError := io.ReadAll(request.Body); readError != nil {
					outcomes <- "gave up on the body"
					return
				}
				outcomes <- "read the whole body"
			})
			listener, listenError := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, listenError)
			server := newServer(config.Load(), handler)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close() })

			connection, dialError := net.Dial("tcp", listener.Addr().String())
			require.NoError(t, dialError)
			t.Cleanup(func() { _ = connection.Close() })
			_, writeError := connection.Write([]byte(testCase.request))
			require.NoError(t, writeError)
			go func() { _, _ = io.Copy(io.Discard, connection) }()

			select {
			case outcome := <-outcomes:
				assert.Equal(t, testCase.expectedOutcome, outcome)
			case <-time.After(10 * time.Second):
				t.Fatal("the handler never finished")
			}
		})
	}
}
