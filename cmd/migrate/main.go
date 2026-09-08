package main

import (
	"context"
	"log"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/joho/godotenv"
)

// The migrate command syncs the PostgreSQL schema with the entity definitions,
// retires whatever stored data has stopped meaning what it used to, and registers the
// markets the system ships knowing about — so that a freshly built database already
// has something to offer and an existing one holds nothing that would be misread.
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

	// 結構同步之後才輪到資料：退場的資料要寫進哪張表，得等那張表存在。
	// A migration is deliberately not interruptible: it is short, it is idempotent,
	// and a half-applied change is worse than one that insists on finishing.
	retiredDataSets, retireError := persistence.
		NewDataRetirementMigrator(database, persistence.NewKCandleRepository(database)).
		Retire(context.Background())
	if retireError != nil {
		log.Fatalf("retiring data failed: %v", retireError)
	}

	if len(retiredDataSets) == 0 {
		log.Print("data retirements: already applied, nothing to retire")
	} else {
		log.Printf("data retirements: applied %d (%s)",
			len(retiredDataSets),
			strings.Join(retiredDataSets, ", "),
		)
	}

	// 建好結構之後才登錄：登錄是業務動作，走 domain，不塞進只管結構的 migrator。
	tradingSymbolApplication := application.NewTradingSymbolApplication(
		// Registering the markets this system ships knowing about reaches no market
		// source: the codes are written into this binary, so there is nothing to
		// confirm with anybody. It is given a router serving no market rather than a
		// stand-in that says yes — if that ever stops being true, this fails loudly
		// instead of quietly registering something no venue has heard of.
		service.NewTradingSymbolService(
			persistence.NewTradingSymbolRepository(database),
			persistence.NewKCandleRepository(database),
			marketdata.NewMarketRoutedSymbolLookupProxy(nil),
			clock.NewSystemClockProxy(),
			domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		),
		// Registering the default markets never touches the watchlist, so nothing here
		// ever asks for a symbol to be caught up. It is given nothing rather than a
		// working ingestion for the same reason as the lookup above: if that ever stops
		// being true, this fails loudly instead of quietly fetching candles from a
		// migration.
		nil,
	)

	registeredSymbols, registerError := tradingSymbolApplication.RegisterDefaultTradingSymbols(
		context.Background())
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
