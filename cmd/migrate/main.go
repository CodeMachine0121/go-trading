package main

import (
	"log"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/joho/godotenv"
)

// The migrate command syncs the PostgreSQL schema with the entity definitions.
func main() {
	if loadError := godotenv.Load(); loadError != nil {
		log.Println("no .env file loaded, falling back to process environment")
	}

	applicationConfig := config.Load()

	database, databaseError := persistence.NewDatabase(applicationConfig.Database.DataSourceName())
	if databaseError != nil {
		log.Fatalf("failed to connect to database: %v", databaseError)
	}

	log.Printf("migrating database %q on %s:%s",
		applicationConfig.Database.Database,
		applicationConfig.Database.Host,
		applicationConfig.Database.Port,
	)

	migratedTables, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	if migrateError != nil {
		log.Fatalf("migration failed: %v", migrateError)
	}

	log.Printf("migration applied to %d table(s): %s",
		len(migratedTables),
		strings.Join(migratedTables, ", "),
	)
}
