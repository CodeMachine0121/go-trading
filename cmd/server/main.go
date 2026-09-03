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

	// The signals are listened for before anything is started, so an interrupt that
	// arrives during the startup backfill is still the one that ends this run rather
	// than being the one that kills it.
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
