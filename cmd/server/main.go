package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if loadError := godotenv.Load(); loadError != nil {
		log.Println("no .env file loaded, falling back to process environment")
	}

	applicationConfig := config.Load()

	database, databaseError := persistence.NewDatabase(applicationConfig.Database.DataSourceName())
	if databaseError != nil {
		log.Fatalf("failed to initialize database: %v", databaseError)
	}

	engine := gin.Default()
	kCandleFollowApplication, kCandleIngestionApplication, strategyBotRunApplication,
		assistantConversationApplication := registerRoutes(engine, database, applicationConfig)

	// An answer being written lives in this process and nowhere else, so every one
	// the last shutdown cut off is stale the moment this one starts. Left alone each
	// is a wait nobody can end, on a conversation nobody can add to.
	//
	// It runs here rather than in the shutdown path because a shutdown is not always
	// given the chance to tidy up: a crash and a power cut leave the same rows behind
	// as a clean stop, and only the next start is guaranteed to happen. Failing to
	// sweep is logged rather than fatal — it leaves some conversations stuck, which
	// is worse than a working system and far better than no system.
	interruptedAnswerCount, sweepError := assistantConversationApplication.FailInterruptedAnswers(
		context.Background())
	if sweepError != nil {
		log.Printf("failed to clear answers interrupted by the last shutdown: %v", sweepError)
	}
	if interruptedAnswerCount > 0 {
		log.Printf("cleared %d assistant answer(s) interrupted by the last shutdown",
			interruptedAnswerCount)
	}

	// The signals are listened for before anything is started, so an interrupt
	// arriving during the startup backfill runs the shutdown path instead of falling
	// back on killing the process. The backfill itself is still cut short — it has
	// not begun watching for a stop that early — but it is cut short through its
	// context, so the calls it has out to the database and the market source end
	// rather than being abandoned mid-flight.
	shutdownSignalled, stopListeningForSignals := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListeningForSignals()

	server := &http.Server{
		Addr:              ":" + applicationConfig.ServerPort,
		Handler:           engine,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	if serveError := serve(
		shutdownSignalled,
		server,
		job.NewBackgroundJobManager(
			backgroundJobsFor(
				applicationConfig,
				kCandleFollowApplication,
				kCandleIngestionApplication,
				strategyBotRunApplication)),
		kCandleFollowApplication.Stop,
	); serveError != nil {
		log.Fatalf("failed to serve: %v", serveError)
	}
}
