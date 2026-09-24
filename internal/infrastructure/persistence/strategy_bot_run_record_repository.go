package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// strategyBotRememberedRunCount is how many rounds one bot remembers.
//
// A bot waking every five minutes runs 288 rounds a day, so a history that kept
// everything would be a table that only ever grows — and the question people ask a
// history is "what has it been doing lately", which fifty rounds answers for every
// interval anybody sets.
//
// Trimming happens on write rather than on a schedule, so there is no second moving
// part that could stop running and let the table grow anyway.
const strategyBotRememberedRunCount = 50

// StrategyBotRunRecordRepository stores what each bot's rounds came to, in
// PostgreSQL.
type StrategyBotRunRecordRepository struct {
	database *gorm.DB
}

func NewStrategyBotRunRecordRepository(database *gorm.DB) *StrategyBotRunRecordRepository {
	return &StrategyBotRunRecordRepository{database: database}
}

// Append records one round and drops whatever fell out of the window.
//
// Both happen in one transaction, because they are two halves of one fact: this bot
// remembers its last fifty rounds. Apart, a failed trim would leave a history that
// grows for ever while every other bot's stays bounded — and nothing would say so.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) Append(
	executionContext context.Context, writeDto dto.StrategyBotRunRecordWriteDto,
) error {
	strategyBotID := writeDto.StrategyBotID

	transactionError := strategyBotRunRecordRepository.database.WithContext(executionContext).
		Transaction(func(transaction *gorm.DB) error {
			latestNumber := 0

			// The number is read rather than counted, so trimming never renumbers
			// anything: Run 51 stays Run 51 once Run 1 is gone. A round that
			// answered to two different names depending on when somebody looked
			// would make a history impossible to talk about.
			//
			// Reading it and writing it are safe together because one bot only ever
			// runs one round at a time.
			if selectError := transaction.
				Model(&entities.StrategyBotRunRecord{}).
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Select("COALESCE(MAX(run_number), 0)").
				Scan(&latestNumber).Error; selectError != nil {
				return selectError
			}

			runRecord := entities.StrategyBotRunRecord{
				StrategyBotID: strategyBotID,
				RunNumber:     latestNumber + 1,
				RanAt:         writeDto.RanAt.UTC(),
				Result:        writeDto.Result,
			}

			// Written only when this round actually suggested something. A round that
			// suggested nothing leaves all three empty, which is a different thing
			// from suggesting zero — and a stop price of zero is a figure somebody
			// really can ask for.
			//
			// An order the venue would have refused is not a suggestion either: the
			// message said so, and there is nothing to place.
			if writeDto.HasPositionPlan && writeDto.PositionPlan.Affordable &&
				!writeDto.PositionPlan.HasVenueRefusal {
				runRecord.SuggestedStake = storedFigure(writeDto.PositionPlan.Stake)

				// A contract round also remembers which way and how far it leaned, so
				// the row reads back the way its message did.
				if writeDto.PositionPlan.ForContract {
					runRecord.SuggestedDirection = writeDto.PositionPlan.Direction
					runRecord.SuggestedLeverage = storedFigure(writeDto.PositionPlan.Leverage)
					runRecord.SuggestedNotional = storedFigure(writeDto.PositionPlan.Notional)
				}

				if writeDto.PositionPlan.HasStopLoss {
					runRecord.SuggestedStopLossPrice = storedFigure(
						writeDto.PositionPlan.StopLossPrice)
				}

				if writeDto.PositionPlan.HasTakeProfit {
					runRecord.SuggestedTakeProfitPrice = storedFigure(
						writeDto.PositionPlan.TakeProfitPrice)
				}
			}

			if createError := transaction.Create(&runRecord).Error; createError != nil {
				return createError
			}

			return transaction.
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Where(clause.Lte{
					Column: "run_number",
					Value:  latestNumber + 1 - strategyBotRememberedRunCount,
				}).
				Delete(&entities.StrategyBotRunRecord{}).Error
		})
	if transactionError != nil {
		return fmt.Errorf("append strategy bot run record: %w", transactionError)
	}

	return nil
}

// storedFigure is one figure this round did suggest, ready to be stored as present
// rather than as a number that happens not to be zero.
func storedFigure(figure decimal.Decimal) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: figure, Valid: true}
}

// FindLatestByBot returns this bot's remembered rounds, newest first.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) FindLatestByBot(
	executionContext context.Context, strategyBotID uint,
) ([]entities.StrategyBotRunRecord, error) {
	runRecords := []entities.StrategyBotRunRecord{}

	result := strategyBotRunRecordRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
		Order("run_number DESC").
		Limit(strategyBotRememberedRunCount).
		Find(&runRecords)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy bot run records: %w", result.Error)
	}

	return runRecords, nil
}
