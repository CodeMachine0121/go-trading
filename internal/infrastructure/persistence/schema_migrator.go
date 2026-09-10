package persistence

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// retiredIndex is an index an entity used to carry. AutoMigrate adds indexes but
// never drops them, so an index that has been replaced stays behind and keeps
// enforcing a rule nobody asked for — which is worse than a leftover column: a
// column just sits there, whereas a leftover unique index refuses writes the system
// now considers perfectly fine.
type retiredIndex struct {
	entity any
	name   string
}

// retiredIndexes are the indexes to drop after the schema is synced. Dropping is
// idempotent, so this list may be kept long after every database has caught up.
var retiredIndexes = []retiredIndex{
	// A strategy's name used to be unique across the whole system. It is now unique
	// within one owner's collection, and the old index would keep the first person
	// here holding "二十根均線" against everybody else forever.
	{entity: &entities.Strategy{}, name: "idx_strategies_name"},
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
		&entities.Session{},
		&entities.PublishedStrategy{},
		&entities.StrategyAdoption{},
	}

	// Clearing has to happen before the schema is synced, not after: a strategy
	// gained an owner that may not be null, and a table with rows in it cannot grow
	// such a column.
	if clearError := schemaMigrator.clearOwnerlessStrategies(); clearError != nil {
		return nil, clearError
	}

	migrateError := schemaMigrator.database.AutoMigrate(migratedEntities...)
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	if dropError := schemaMigrator.dropRetiredColumns(); dropError != nil {
		return nil, dropError
	}

	if dropError := schemaMigrator.dropRetiredIndexes(); dropError != nil {
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

// dropRetiredIndexes removes every index no entity claims any more, skipping the
// ones already gone so that running this twice is the same as running it once.
func (schemaMigrator *SchemaMigrator) dropRetiredIndexes() error {
	migrator := schemaMigrator.database.Migrator()

	for _, index := range retiredIndexes {
		if !migrator.HasIndex(index.entity, index.name) {
			continue
		}

		// The ORM's own DropIndex is not usable here, and this is the one place in
		// the codebase that writes a statement out by hand. On this driver it
		// builds "DROP INDEX <schema>.<name>" and, on a connection that names no
		// schema, fills the first blank with a function call — which is not valid
		// there. The statement below is what it was trying to write.
		//
		// It carries no value from anywhere: the name is a constant in the list
		// above, and it goes through the ORM's own identifier quoting rather than
		// being pasted into the text.
		dropped := schemaMigrator.database.Exec("DROP INDEX IF EXISTS ?", clause.Column{Name: index.name})
		if dropped.Error != nil {
			return fmt.Errorf("drop retired index %s: %w", index.name, dropped.Error)
		}
	}

	return nil
}

// clearOwnerlessStrategies drops every strategy saved before strategies belonged to
// anybody. Their knobs go with them through the cascade already on the table.
//
// The condition is the point: it fires only while the Strategies table exists and
// has no owner column, which is exactly once, and never again after the migration
// that follows it. It is therefore not a script somebody has to remember to run
// once — it is a statement about a shape that stops being true the moment it has
// done its work, in the same spirit as the retired columns above.
//
// Assigning the rows to somebody instead was the alternative, and it was rejected:
// picking an owner for a test row is a guess, and a guess here would leave "every
// strategy has an owner" true only by accident.
func (schemaMigrator *SchemaMigrator) clearOwnerlessStrategies() error {
	migrator := schemaMigrator.database.Migrator()
	if !migrator.HasTable(&entities.Strategy{}) {
		return nil
	}

	if migrator.HasColumn(&entities.Strategy{}, "owner_id") {
		return nil
	}

	if deleteError := schemaMigrator.database.
		Where("1 = 1").
		Delete(&entities.Strategy{}).Error; deleteError != nil {
		return fmt.Errorf("clear ownerless strategies: %w", deleteError)
	}

	return nil
}
