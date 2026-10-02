package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Checked first: a script compartment serves one request and reads no settings or database.
	if len(os.Args) > 1 && os.Args[1] == script.IndicatorScriptWorkerCommand {
		os.Exit(script.NewIndicatorScriptWorker().Serve(os.Stdin, os.Stdout))
	}

	if loadError := godotenv.Load(); loadError != nil {
		log.Println("no .env file loaded, falling back to process environment")
	}

	applicationConfig := config.Load()

	database, databaseError := persistence.NewDatabase(applicationConfig.Database.DataSourceName())
	if databaseError != nil {
		log.Fatalf("failed to initialize database: %v", databaseError)
	}

	engine := gin.Default()
	liveFollows, kCandleIngestionApplication, kCandleContractIngestionApplication,
		strategyBotJobs, interruptedWorkApplication,
		contractSeries := registerRoutes(engine, database, applicationConfig)

	// Said alive before sweeping, then whatever is still running without a live replica behind it is marked failed:
	// a crash never reaches a shutdown hook, and other replicas' work in flight is left alone. A failure is logged, not fatal.
	if beatError := interruptedWorkApplication.BeatHeartbeat(context.Background()); beatError != nil {
		log.Printf("failed to record this replica's heartbeat at startup: %v", beatError)
	}
	interrupted, sweepError := interruptedWorkApplication.FailWorkLeftByLastRun(context.Background())
	if sweepError != nil {
		log.Printf("failed to clear work interrupted by a vanished replica: %v", sweepError)
	}
	if interrupted != (dto.InterruptedWorkDto{}) {
		log.Printf("cleared work interrupted by a vanished replica: %d answer(s), %d history sync(s), "+
			"%d contract history sync(s)", interrupted.Answers, interrupted.HistorySyncs, interrupted.ContractHistorySyncs)
	}

	// Listened for before anything starts, so an interrupt during the startup backfill takes the
	// shutdown path and cancels in-flight calls through context instead of killing the process.
	shutdownSignalled, stopListeningForSignals := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListeningForSignals()

	// Only a replica that runs background jobs may hold the duty, or it would hold it while doing nothing.
	jobLeadershipApplication := jobLeadershipApplicationFor(database, applicationConfig)
	dutyHolder := jobLeadershipApplication
	if !applicationConfig.BackgroundJobsEnabled {
		dutyHolder = nil
		// Bot messages are queued by any replica but sent only by one running background jobs.
		log.Println("background jobs are off on this replica: it queues bot messages but sends none, " +
			"so at least one replica must run with background jobs on")
	}

	if serveError := serve(
		shutdownSignalled,
		newServer(applicationConfig, engine),
		job.NewBackgroundJobManager(
			backgroundJobsFor(
				applicationConfig,
				jobLeadershipApplication,
				interruptedWorkApplication,
				liveFollows.spot,
				kCandleIngestionApplication,
				kCandleContractIngestionApplication,
				strategyBotJobs,
				contractSeries)),
		liveFollows.Stop,
		dutyHolder,
	); serveError != nil {
		log.Fatalf("failed to serve: %v", serveError)
	}
}
