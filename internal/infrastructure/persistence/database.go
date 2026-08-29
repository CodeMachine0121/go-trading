package persistence

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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

	// Every entity whose table is managed code first is registered here.
	migrateError := database.AutoMigrate(&entities.KCandle{})
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	return database, nil
}
