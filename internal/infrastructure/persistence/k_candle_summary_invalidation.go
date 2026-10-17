package persistence

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// kCandleSummaryInvalidation voids the days a minute write touched inside that write's transaction; it sits with the minute repositories because every minute write path ends in them.
type kCandleSummaryInvalidation[TSummaryDay any, TSummary any] struct{}

func (kCandleSummaryInvalidation[TSummaryDay, TSummary]) invalidate(
	transaction *gorm.DB, symbol string, openTimes []time.Time,
) error {
	summaryIntervalValues := make([]any, 0, 2)
	for _, intervalValue := range domains.KCandleSummaryIntervalValues() {
		summaryIntervalValues = append(summaryIntervalValues, string(intervalValue))
	}

	invalidatedDays := make(map[time.Time]bool, 1)
	for _, openTime := range openTimes {
		day := openTime.UTC().Truncate(domains.KCandleSummaryDayLength)
		if invalidatedDays[day] {
			continue
		}
		invalidatedDays[day] = true

		// GORM takes an untyped row here because the day row's type is a type parameter.
		stateResult := transaction.Model(new(TSummaryDay)).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "symbol"}, {Name: "day"}},
				DoUpdates: clause.Set{
					// The increment must read the stored row inside the upsert; GORM has no typed form for it.
					{Column: clause.Column{Name: "revision"}, Value: clause.Expr{
						SQL: "? + 1", Vars: []any{clause.Column{Table: clause.CurrentTable, Name: "revision"}}}},
					{Column: clause.Column{Name: "summarized"}, Value: false},
				},
			}).
			Create(map[string]any{"symbol": symbol, "day": day, "revision": 1, "summarized": false})
		if stateResult.Error != nil {
			return fmt.Errorf("invalidate k candle summary day: %w", stateResult.Error)
		}

		summaryResult := transaction.
			Where(clause.Eq{Column: "symbol", Value: symbol}).
			// Naming the intervals lets the (symbol, interval, open_time) index bound the delete to this day.
			Where(clause.IN{Column: "interval", Values: summaryIntervalValues}).
			Where(clause.Gte{Column: "open_time", Value: day}).
			Where(clause.Lt{Column: "open_time", Value: day.Add(domains.KCandleSummaryDayLength)}).
			Delete(new(TSummary))
		if summaryResult.Error != nil {
			return fmt.Errorf("delete invalidated k candle summaries: %w", summaryResult.Error)
		}
	}

	return nil
}
