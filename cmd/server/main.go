package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeMachine0121/go-trading/internal/config"
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
		strategyBotRunApplication, assistantConversationApplication,
		contractSeries := registerRoutes(engine, database, applicationConfig)

	// In-flight answers live only in this process, so any left by the last run are stale; swept at
	// startup because a crash never reaches a shutdown hook, and a failed sweep is logged, not fatal.
	interruptedAnswerCount, sweepError := assistantConversationApplication.FailInterruptedAnswers(
		context.Background())
	if sweepError != nil {
		log.Printf("failed to clear answers interrupted by the last shutdown: %v", sweepError)
	}
	if interruptedAnswerCount > 0 {
		log.Printf("cleared %d assistant answer(s) interrupted by the last shutdown",
			interruptedAnswerCount)
	}

	// History syncs are driven by this process too, so any still marked fetching are orphaned.
	interruptedSyncCount, syncSweepError := kCandleIngestionApplication.FailInterruptedHistorySyncs(
		context.Background())
	if syncSweepError != nil {
		log.Printf("failed to clear history syncs interrupted by the last shutdown: %v",
			syncSweepError)
	}
	if interruptedSyncCount > 0 {
		log.Printf("cleared %d k candle history sync(s) interrupted by the last shutdown",
			interruptedSyncCount)
	}

	// Contract syncs live in their own table, so the sweep above cannot see them.
	interruptedContractSyncCount, contractSyncSweepError := kCandleContractIngestionApplication.
		FailInterruptedHistorySyncs(context.Background())
	if contractSyncSweepError != nil {
		log.Printf("failed to clear contract history syncs interrupted by the last shutdown: %v",
			contractSyncSweepError)
	}
	if interruptedContractSyncCount > 0 {
		log.Printf("cleared %d contract k candle history sync(s) interrupted by the last shutdown",
			interruptedContractSyncCount)
	}

	// Listened for before anything starts, so an interrupt during the startup backfill takes the
	// shutdown path and cancels in-flight calls through context instead of killing the process.
	shutdownSignalled, stopListeningForSignals := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListeningForSignals()

	if serveError := serve(
		shutdownSignalled,
		newServer(applicationConfig, engine),
		job.NewBackgroundJobManager(
			backgroundJobsFor(
				applicationConfig,
				liveFollows.spot,
				kCandleIngestionApplication,
				kCandleContractIngestionApplication,
				strategyBotRunApplication,
				contractSeries)),
		liveFollows.Stop,
	); serveError != nil {
		log.Fatalf("failed to serve: %v", serveError)
	}
}
