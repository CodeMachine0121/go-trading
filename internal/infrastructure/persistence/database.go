package persistence

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDatabase opens the PostgreSQL connection and syncs the schema code first.
// Register every new entity in the AutoMigrate call below; no DDL is hand written.
func NewDatabase(dataSourceName string) (*gorm.DB, error) {
	database, openError := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{})
	if openError != nil {
		return nil, fmt.Errorf("open postgres connection: %w", openError)
	}

	// Entities from internal/domain/models/entities go here, e.g. &entities.Order{}.
	migrateError := database.AutoMigrate()
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	return database, nil
}
