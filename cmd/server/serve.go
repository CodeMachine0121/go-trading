package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
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

// leadershipCallTimeout bounds the duty calls made outside any job, so a stalled database cannot hold up start or exit.
const leadershipCallTimeout = 5 * time.Second

// shutdownGrace bounds request draining only; background rounds are cut off when it ends, which is
// safe because the next startup backfill closes any candle gap.
const shutdownGrace = 15 * time.Second

// serve runs the HTTP server and background jobs until shutdown is signalled: jobs stop taking rounds,
// live follows end, requests drain, and only then is the jobs' own context cancelled.
// stopLiveFollows must run before draining, since each follow holds a request open and would
// otherwise make every shutdown wait out the full grace period.
// jobLeadership, nil on a replica without background jobs, is renewed once before the jobs start so
// their first round already knows whether this replica is on duty, and released only after every job
// round has been cut off, so the next replica on duty never overlaps this one.
func serve(
	shutdownSignalled context.Context,
	server *http.Server,
	backgroundJobManager *job.BackgroundJobManager,
	stopLiveFollows func(),
	jobLeadership *application.JobLeadershipApplication,
) error {
	backgroundJobWork, giveUpOnBackgroundJobWork := context.WithCancel(context.Background())
	defer giveUpOnBackgroundJobWork()
	defer releaseLeadership(jobLeadership, giveUpOnBackgroundJobWork)

	if jobLeadership != nil {
		renewal, endRenewal := context.WithTimeout(context.Background(), leadershipCallTimeout)
		if _, renewError := jobLeadership.RenewLeadership(renewal); renewError != nil {
			log.Printf("job leadership could not be taken at startup: %v", renewError)
		}
		endRenewal()
	}

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

// releaseLeadership cuts off the job rounds first and only then gives the duty back.
func releaseLeadership(
	jobLeadership *application.JobLeadershipApplication, giveUpOnBackgroundJobWork context.CancelFunc,
) {
	if jobLeadership == nil {
		return
	}

	giveUpOnBackgroundJobWork()

	release, endRelease := context.WithTimeout(context.Background(), leadershipCallTimeout)
	defer endRelease()

	if releaseError := jobLeadership.ReleaseLeadership(release); releaseError != nil {
		log.Printf("job leadership could not be given back; the next replica waits out the lease: %v",
			releaseError)
	}
}
