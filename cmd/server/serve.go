package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/job"
)

// readHeaderTimeout stops idle connections that never send headers from holding a slot forever.
const readHeaderTimeout = 10 * time.Second

// newServer sets no WriteTimeout: it is a deadline on the whole response, which would cut off live streams
// and long backtests, and slow readers are absorbed by the reverse proxy in front.
func newServer(applicationConfig config.ApplicationConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + applicationConfig.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       applicationConfig.RequestLimit.ReadTimeout,
		IdleTimeout:       applicationConfig.RequestLimit.IdleTimeout,
	}
}

// shutdownGrace bounds request draining only; background rounds are cut off when it ends, which is
// safe because the next startup backfill closes any candle gap.
const shutdownGrace = 15 * time.Second

// serve runs the HTTP server and background jobs until shutdown is signalled: jobs stop taking rounds,
// live follows end, requests drain, and only then is the jobs' own context cancelled.
// stopLiveFollows must run before draining, since each follow holds a request open and would
// otherwise make every shutdown wait out the full grace period.
func serve(
	shutdownSignalled context.Context,
	server *http.Server,
	backgroundJobManager *job.BackgroundJobManager,
	stopLiveFollows func(),
) error {
	backgroundJobWork, giveUpOnBackgroundJobWork := context.WithCancel(context.Background())
	defer giveUpOnBackgroundJobWork()

	backgroundJobManager.StartAll(backgroundJobWork)

	// Buffered so the goroutine can exit even if shutdown won the race and nobody reads it.
	listenFailures := make(chan error, 1)
	go func() { listenFailures <- server.ListenAndServe() }()

	select {
	case listenError := <-listenFailures:
		// The server stopping on its own is a failure, but jobs are still stopped before returning.
		backgroundJobManager.StopAll()
		stopLiveFollows()

		return fmt.Errorf("listen on %s: %w", server.Addr, listenError)
	case <-shutdownSignalled.Done():
	}

	backgroundJobManager.StopAll()
	stopLiveFollows()

	drainRequests, stopDraining := context.WithTimeout(context.Background(), shutdownGrace)
	defer stopDraining()

	if shutdownError := server.Shutdown(drainRequests); shutdownError != nil {
		return fmt.Errorf("shut down cleanly within %s: %w", shutdownGrace, shutdownError)
	}

	return nil
}
