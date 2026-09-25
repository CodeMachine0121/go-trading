package persistence

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// slowQueryThreshold matches GORM's default; the custom logger exists only for the not-found flag.
const slowQueryThreshold = 200 * time.Millisecond

// NewDatabase opens the PostgreSQL connection without touching the schema, and keeps "record not found" out of the log because empty reads are expected answers here.
func NewDatabase(dataSourceName string) (*gorm.DB, error) {
	database, openError := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             slowQueryThreshold,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	})
	if openError != nil {
		return nil, fmt.Errorf("open postgres connection: %w", openError)
	}

	return database, nil
}
