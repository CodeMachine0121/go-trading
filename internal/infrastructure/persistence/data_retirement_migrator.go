package persistence

import (
	"context"
	"fmt"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
)

// KCandlesBeforeOneMinuteGranularityRetirement names the one-off that empties the K
// candles stored back when one of them covered five minutes.
//
// Nothing about such a candle looks different from a one-minute one — same columns,
// same shape, an open time that lands on a whole minute either way. Left in place
// they would be read as one-minute candles by indicator calculation and by backtests,
// and the figures that came out would be wrong in a way nobody could see. So they go,
// and the startup backfill fetches the range again at the new length.
//
// The name is written out rather than built, because it is what "already done" is
// decided by: change it and every database does this again.
const KCandlesBeforeOneMinuteGranularityRetirement = "k-candles-before-one-minute-granularity"

// retiredDataSet is a body of stored data that no longer means what it used to, and
// the work of getting rid of it.
//
// It sits beside retiredColumns for the same reason that one exists: a change the
// schema cannot express is a change somebody has to be able to read. A column left
// behind misleads whoever reads the table; data left behind misleads whoever computes
// from it.
type retiredDataSet struct {
	name   string
	retire func(executionContext context.Context) (int64, error)
}

// DataRetirementMigrator applies the one-off data retirements that have not been
// applied yet, and remembers which ones it applied.
//
// Remembering is the whole point. Schema changes are idempotent because the schema
// says what it already is; a body of data cannot say that about itself, so a run that
// forgot would empty the candles the previous run went and fetched.
type DataRetirementMigrator struct {
	database        *gorm.DB
	retiredDataSets []retiredDataSet
}

// NewDataRetirementMigrator declares every retirement this system knows about.
// Adding the next one is adding a row here.
func NewDataRetirementMigrator(
	database *gorm.DB, kCandleRepository _interface.IKCandleRepository,
) *DataRetirementMigrator {
	return &DataRetirementMigrator{
		database: database,
		retiredDataSets: []retiredDataSet{
			{
				name:   KCandlesBeforeOneMinuteGranularityRetirement,
				retire: kCandleRepository.DeleteAll,
			},
		},
	}
}

// Retire applies whichever retirements have not run here before, and reports their
// names in the order they were applied. A run with nothing left to do reports nothing
// and is not a failure.
//
// Each retirement does its work first and is recorded after. The other order reads as
// the safer one and is not: a run that stopped between the two would have remembered
// a retirement it never carried out, leaving data nothing will ever remove again.
// This way the worst a stop between them costs is doing it once more next time, which
// the startup backfill undoes by fetching the range again.
//
// Not knowing whether a retirement has run stops the whole thing rather than being
// read as "not yet" — guessing that way would empty a store already refilled.
func (dataRetirementMigrator *DataRetirementMigrator) Retire(
	executionContext context.Context,
) ([]string, error) {
	appliedNames := make([]string, 0, len(dataRetirementMigrator.retiredDataSets))

	for _, retiredData := range dataRetirementMigrator.retiredDataSets {
		// Counting rather than fetching: the row's contents are of no interest, and
		// asking for one that is not there is the ordinary case here — reading that as
		// the absence of a record rather than as a failure to find one keeps a first
		// run from looking like something went wrong.
		appliedCount := int64(0)
		lookupError := dataRetirementMigrator.database.WithContext(executionContext).
			Model(&entities.AppliedDataRetirement{}).
			Where(&entities.AppliedDataRetirement{Name: retiredData.name}).
			Count(&appliedCount).Error
		if lookupError != nil {
			return nil, fmt.Errorf("read applied data retirements: %w", lookupError)
		}
		if appliedCount > 0 {
			continue
		}

		if _, retireError := retiredData.retire(executionContext); retireError != nil {
			return nil, fmt.Errorf("apply data retirement %s: %w", retiredData.name, retireError)
		}

		if recordError := dataRetirementMigrator.database.WithContext(executionContext).
			Create(&entities.AppliedDataRetirement{
				Name: retiredData.name, AppliedAt: time.Now().UTC(),
			}).Error; recordError != nil {
			return nil, fmt.Errorf("record data retirement %s: %w", retiredData.name, recordError)
		}

		appliedNames = append(appliedNames, retiredData.name)
	}

	return appliedNames, nil
}
