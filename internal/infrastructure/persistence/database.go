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

// slowQueryThreshold is how long a query may take before it is worth a line in the
// log. It matches GORM's own default — the reason this logger is built by hand is
// the not-found flag below, not this number.
const slowQueryThreshold = 200 * time.Millisecond

// NewDatabase opens the PostgreSQL connection. It never touches the schema;
// schema sync is the migrate command's job (see SchemaMigrator).
//
// "Record not found" is kept out of the log, because in this system it is an
// answer rather than a failure: every read that can come back empty translates it
// into one — a strategy that is not on the marketplace, a person with no delivery
// setting, a bot that is not there. The program still receives the error and still
// has to handle it; only the line saying so is dropped.
//
// Without this, one standing bot fills the log with a stack of "record not found"
// every few minutes, all of them describing conditions the code deliberately
// expects — and a log that is mostly expected conditions is a log nobody reads the
// unexpected ones out of.
func NewDatabase(dataSourceName string) (*gorm.DB, error) {
	database, openError := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold: slowQueryThreshold,
				LogLevel:      logger.Warn,
				// The one line this whole block exists for.
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
