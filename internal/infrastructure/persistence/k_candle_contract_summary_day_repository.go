package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KCandleContractSummaryDayRepository struct {
	database *gorm.DB
}

func NewKCandleContractSummaryDayRepository(database *gorm.DB) *KCandleContractSummaryDayRepository {
	return &KCandleContractSummaryDayRepository{database: database}
}

func (kCandleContractSummaryDayRepository *KCandleContractSummaryDayRepository) FindDaysInRange(
	executionContext context.Context,
	symbol string,
	interval vo.AggregationIntervalVo,
	startTime time.Time,
	endTime time.Time,
) ([]entities.KCandleContractSummaryDay, error) {
	summaryDays := make([]entities.KCandleContractSummaryDay, 0)
	summaries := make([]entities.KCandleContractSummary, 0)

	// One snapshot for both reads, or an invalidation landing between them would show a summarized day with its summaries already gone.
	transactionError := kCandleContractSummaryDayRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			dayResult := transaction.
				Where(clause.Eq{Column: "symbol", Value: symbol}).
				Where(clause.Gte{Column: "day", Value: startTime.UTC().Truncate(domains.KCandleSummaryDayLength)}).
				Where(clause.Lt{Column: "day", Value: endTime.UTC()}).
				Order("day").
				Find(&summaryDays)
			if dayResult.Error != nil {
				return fmt.Errorf("find contract k candle summary days: %w", dayResult.Error)
			}

			summaryResult := transaction.
				Where(clause.Eq{Column: "symbol", Value: symbol}).
				Where(clause.Eq{Column: "interval", Value: string(interval)}).
				Where(clause.Gte{Column: "open_time", Value: startTime.UTC()}).
				Where(clause.Lt{Column: "open_time", Value: endTime.UTC()}).
				Order("open_time").
				Find(&summaries)
			if summaryResult.Error != nil {
				return fmt.Errorf("find contract k candle summaries: %w", summaryResult.Error)
			}

			return nil
		}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if transactionError != nil {
		return nil, transactionError
	}

	summariesByDay := make(map[time.Time][]entities.KCandleContractSummary, len(summaryDays))
	for _, summary := range summaries {
		day := summary.OpenTime.UTC().Truncate(domains.KCandleSummaryDayLength)
		summariesByDay[day] = append(summariesByDay[day], summary)
	}
	for index := range summaryDays {
		summaryDays[index].Day = summaryDays[index].Day.UTC()
		if summaryDays[index].Summarized {
			summaryDays[index].Summaries = summariesByDay[summaryDays[index].Day]
		}
	}

	return summaryDays, nil
}

func (kCandleContractSummaryDayRepository *KCandleContractSummaryDayRepository) SaveSummarized(
	executionContext context.Context, summaryDay entities.KCandleContractSummaryDay,
) (bool, error) {
	isStored := false

	transactionError := kCandleContractSummaryDayRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// One statement claims the day both when it has no state row yet and when its row is still at the revision the merge read.
			stateResult := transaction.
				Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "symbol"}, {Name: "day"}},
					DoUpdates: clause.Assignments(map[string]any{"summarized": true}),
					Where: clause.Where{Exprs: []clause.Expression{
						clause.Eq{Column: clause.Column{
							Table: entities.KCandleContractSummaryDay{}.TableName(), Name: "revision"}, Value: summaryDay.Revision},
						clause.Eq{Column: clause.Column{
							Table: entities.KCandleContractSummaryDay{}.TableName(), Name: "summarized"}, Value: false},
					}},
				}).
				Create(&entities.KCandleContractSummaryDay{
					Symbol:     summaryDay.Symbol,
					Day:        summaryDay.Day.UTC(),
					Revision:   summaryDay.Revision,
					Summarized: true,
				})
			if stateResult.Error != nil {
				return fmt.Errorf("claim contract k candle summary day: %w", stateResult.Error)
			}
			if stateResult.RowsAffected == 0 {
				return nil
			}

			if len(summaryDay.Summaries) > 0 {
				summaryResult := transaction.Create(&summaryDay.Summaries)
				if summaryResult.Error != nil {
					return fmt.Errorf("save contract k candle summaries: %w", summaryResult.Error)
				}
			}
			isStored = true

			return nil
		})
	if transactionError != nil {
		return false, transactionError
	}

	return isStored, nil
}
