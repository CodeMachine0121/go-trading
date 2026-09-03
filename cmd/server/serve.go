package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/job"
)

// readHeaderTimeout bounds how long a client may take to send its request headers.
// Without it a connection that opens and then says nothing holds a slot for as long
// as it likes, which is a way to take the server down that costs the caller nothing.
const readHeaderTimeout = 10 * time.Second

// shutdownGrace is how long the requests already accepted are given to be answered
// once shutdown has begun. Past it they are abandoned, because a shutdown that can
// be refused is not a shutdown.
//
// It bounds the requests only. A background round part way through storing candles
// is asked to stop and is then cut off when this draining ends — which, with nothing
// left to drain, is at once. Giving a round a window of its own would mean waiting
// for one, and a job cannot yet say when it has finished. Nothing is lost by cutting
// it: candles are stored one at a time and the next startup backfill closes whatever
// gap the cut left, which is the whole reason a backfill runs first.
const shutdownGrace = 15 * time.Second

// serve runs the HTTP server and the background jobs until shutdown is signalled,
// then lets what is in flight finish before returning.
//
// The way down is not the way up reversed but something more particular, because
// "stop" and "give up" are different requests and the jobs are told them in that
// order. First they are asked to take on no further rounds, so nothing new starts
// while requests are still being answered. Then the server is asked to close, which
// waits for the requests it already accepted. Only once that wait is over — however
// it ended — is the jobs' own context let go of, which reaches the calls they have
// out to the database and the market source and abandons them.
//
// The jobs therefore run under a context of their own rather than under the signal:
// were they started under it, an interrupt would abandon a round in the same instant
// it asked the round to finish, and the asking would mean nothing.
func serve(
	shutdownSignalled context.Context,
	server *http.Server,
	backgroundJobManager *job.BackgroundJobManager,
) error {
	backgroundJobWork, giveUpOnBackgroundJobWork := context.WithCancel(context.Background())
	defer giveUpOnBackgroundJobWork()

	backgroundJobManager.StartAll(backgroundJobWork)

	// Buffered, so the goroutine can report a failure and finish even if nobody is
	// left listening because shutdown won the race.
	listenFailures := make(chan error, 1)
	go func() { listenFailures <- server.ListenAndServe() }()

	select {
	case listenError := <-listenFailures:
		// The server stopping on its own is a failure to serve, not a shutdown —
		// except for the one error Shutdown itself causes, which cannot arrive here
		// because nothing has asked for it yet. The jobs are stopped all the same:
		// leaving this function with work still being scheduled is never right,
		// however it is left.
		backgroundJobManager.StopAll()

		return fmt.Errorf("listen on %s: %w", server.Addr, listenError)
	case <-shutdownSignalled.Done():
	}

	backgroundJobManager.StopAll()

	drainRequests, stopDraining := context.WithTimeout(context.Background(), shutdownGrace)
	defer stopDraining()

	if shutdownError := server.Shutdown(drainRequests); shutdownError != nil {
		return fmt.Errorf("shut down cleanly within %s: %w", shutdownGrace, shutdownError)
	}

	return nil
}
