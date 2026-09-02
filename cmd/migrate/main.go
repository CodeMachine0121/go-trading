package main

import (
	"log"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/joho/godotenv"
)

// The migrate command syncs the PostgreSQL schema with the entity definitions and
// registers the markets the system ships knowing about, so that a freshly built
// database already has something to offer.
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

	// 建好結構之後才登錄：登錄是業務動作，走 domain，不塞進只管結構的 migrator。
	tradingSymbolApplication := application.NewTradingSymbolApplication(
		service.NewTradingSymbolService(
			persistence.NewTradingSymbolRepository(database),
			persistence.NewKCandleRepository(database),
		),
	)

	registeredSymbols, registerError := tradingSymbolApplication.RegisterDefaultTradingSymbols()
	if registerError != nil {
		log.Fatalf("registering the default trading symbols failed: %v", registerError)
	}

	if len(registeredSymbols) == 0 {
		log.Print("default trading symbols: already registered, nothing to add")
		return
	}

	log.Printf("default trading symbols: registered %d new (%s)",
		len(registeredSymbols),
		strings.Join(registeredSymbols, ", "),
	)
}
