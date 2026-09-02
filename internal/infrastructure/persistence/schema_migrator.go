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

// Migrate creates or updates the table of every registered entity and reports the
// resulting table names. Register every new entity in the slice below.
func (schemaMigrator *SchemaMigrator) Migrate() ([]string, error) {
	migratedEntities := []any{
		&entities.KCandle{},
		&entities.TradingSymbol{},
	}

	migrateError := schemaMigrator.database.AutoMigrate(migratedEntities...)
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
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
