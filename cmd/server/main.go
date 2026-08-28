package main

import (
	"log"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if loadError := godotenv.Load(); loadError != nil {
		log.Println("no .env file loaded, falling back to process environment")
	}

	serverConfig := loadServerConfig()

	database, databaseError := persistence.NewDatabase(serverConfig.Database.DataSourceName())
	if databaseError != nil {
		log.Fatalf("failed to initialize database: %v", databaseError)
	}

	engine := gin.Default()
	registerRoutes(engine, database)

	if runError := engine.Run(":" + serverConfig.ServerPort); runError != nil {
		log.Fatalf("failed to start server: %v", runError)
	}
}
