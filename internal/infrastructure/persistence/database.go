package persistence

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDatabase opens the PostgreSQL connection. It never touches the schema;
// schema sync is the migrate command's job (see SchemaMigrator).
func NewDatabase(dataSourceName string) (*gorm.DB, error) {
	database, openError := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{})
	if openError != nil {
		return nil, fmt.Errorf("open postgres connection: %w", openError)
	}

	return database, nil
}
