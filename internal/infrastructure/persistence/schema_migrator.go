package persistence

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
)

// SchemaMigrator syncs the database schema from the entity definitions, code first.
// The loose element type is required by GORM's AutoMigrate signature and stays
// confined to this infrastructure file.
type SchemaMigrator struct {
	database *gorm.DB
}

func NewSchemaMigrator(database *gorm.DB) *SchemaMigrator {
	return &SchemaMigrator{database: database}
}

// retiredColumn is a column an entity used to have. AutoMigrate adds and widens but
// never drops, so a field removed from an entity leaves its column behind forever
// unless it is said out loud here — and a reader who finds aggregation_interval
// still sitting on Strategies has every reason to believe a strategy still
// remembers it.
type retiredColumn struct {
	entity any
	name   string
}

// retiredColumns are the columns to drop after the schema is synced. Dropping is
// idempotent: a column that is already gone is skipped, so this list may be kept
// long after every database has caught up.
var retiredColumns = []retiredColumn{
	// How coarse the K candles are and how many of them describe one run of an
	// algorithm, not the algorithm; they moved onto the calculation request.
	{entity: &entities.Strategy{}, name: "aggregation_interval"},
	{entity: &entities.Strategy{}, name: "candle_count"},
}

// Migrate creates or updates the table of every registered entity, drops the columns
// no entity claims any more, and reports the resulting table names. Register every
// new entity in the slice below.
func (schemaMigrator *SchemaMigrator) Migrate() ([]string, error) {
	migratedEntities := []any{
		&entities.KCandle{},
		&entities.TradingSymbol{},
		&entities.Strategy{},
		&entities.StrategyParameter{},
		&entities.Conversation{},
		&entities.AssistantTurn{},
		&entities.AssistantQueryRecord{},
		&entities.User{},
	}

	migrateError := schemaMigrator.database.AutoMigrate(migratedEntities...)
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	if dropError := schemaMigrator.dropRetiredColumns(); dropError != nil {
		return nil, dropError
	}

	migratedTables := make([]string, 0, len(migratedEntities))
	for _, migratedEntity := range migratedEntities {
		statement := &gorm.Statement{DB: schemaMigrator.database}
		if parseError := statement.Parse(migratedEntity); parseError != nil {
			return nil, fmt.Errorf("resolve migrated table name: %w", parseError)
		}
		migratedTables = append(migratedTables, statement.Schema.Table)
	}

	return migratedTables, nil
}

// dropRetiredColumns removes every column no entity claims any more, skipping the
// ones already gone so that running this twice is the same as running it once.
func (schemaMigrator *SchemaMigrator) dropRetiredColumns() error {
	migrator := schemaMigrator.database.Migrator()

	for _, column := range retiredColumns {
		if !migrator.HasColumn(column.entity, column.name) {
			continue
		}

		if dropError := migrator.DropColumn(column.entity, column.name); dropError != nil {
			return fmt.Errorf("drop retired column %s: %w", column.name, dropError)
		}
	}

	return nil
}
