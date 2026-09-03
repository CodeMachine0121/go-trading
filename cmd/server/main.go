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
	registerRoutes(engine, database, applicationConfig)

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
		job.NewBackgroundJobManager(backgroundJobsFor(database, applicationConfig)),
	); serveError != nil {
		log.Fatalf("failed to serve: %v", serveError)
	}
}
