package persistence

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDatabase opens the PostgreSQL connection. It never touches the schema;
// schema sync is the migrate command's job (see SchemaMigrator).
//
// Errors are translated so that a repository can recognise a broken constraint by
// what it is — gorm.ErrDuplicatedKey — rather than by matching the driver's message
// text, which differs per database and per version and fails silently when it drifts.
func NewDatabase(dataSourceName string) (*gorm.DB, error) {
	database, openError := gorm.Open(
		postgres.Open(dataSourceName), &gorm.Config{TranslateError: true})
	if openError != nil {
		return nil, fmt.Errorf("open postgres connection: %w", openError)
	}

	return database, nil
}
