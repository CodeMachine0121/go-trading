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

// strategyBotRememberedRunCount caps each bot's run history, trimmed on write so no separate cleanup job is needed.
const strategyBotRememberedRunCount = 50

// It reads through ambientTransactionDatabase so a booked round can land in one transaction with what it caused.
type StrategyBotRunRecordRepository struct {
	database ambientTransactionDatabase
}

func NewStrategyBotRunRecordRepository(database *gorm.DB) *StrategyBotRunRecordRepository {
	return &StrategyBotRunRecordRepository{database: ambientTransactionDatabase{root: database}}
}

// Append inserts the run and trims the history in one transaction, so a failed trim cannot let one bot's history grow unbounded.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) Append(
	executionContext context.Context, writeDto dto.StrategyBotRunRecordWriteDto,
) error {
	strategyBotID := writeDto.StrategyBotID

	transactionError := strategyBotRunRecordRepository.database.within(executionContext).
		Transaction(func(transaction *gorm.DB) error {
			latestNumber := 0

			// The next run number is read from the latest rather than counted, so trimming never renumbers runs; safe because a bot runs one round at a time.
			if selectError := transaction.
				Model(&entities.StrategyBotRunRecord{}).
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Select("COALESCE(MAX(run_number), 0)").
				Scan(&latestNumber).Error; selectError != nil {
				return selectError
			}

			runRecord := entities.StrategyBotRunRecord{
				StrategyBotID:         strategyBotID,
				RunNumber:             latestNumber + 1,
				RanAt:                 writeDto.RanAt.UTC(),
				Result:                writeDto.Result,
				ReferencePrice:        writeDto.ReferencePrice,
				JournalLinkIdentifier: writeDto.JournalLinkIdentifier,
			}

			// Plan figures are stored only for an affordable suggestion, leaving them null otherwise, since zero is a valid stop price.
			if writeDto.HasPositionPlan && writeDto.PositionPlan.Affordable &&
				!writeDto.PositionPlan.HasVenueRefusal {
				runRecord.SuggestedStake = storedFigure(writeDto.PositionPlan.Stake)

				// Contract runs also store direction and leverage.
				if writeDto.PositionPlan.ForContract {
					runRecord.SuggestedDirection = writeDto.PositionPlan.Direction
					runRecord.SuggestedLeverage = storedFigure(writeDto.PositionPlan.Leverage)
					runRecord.SuggestedNotional = storedFigure(writeDto.PositionPlan.Notional)
					if writeDto.PositionPlan.HasQuantity {
						runRecord.SuggestedQuantity = storedFigure(writeDto.PositionPlan.Quantity)
					}
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

// storedFigure marks a suggested figure as present, even if zero.
func storedFigure(figure decimal.Decimal) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: figure, Valid: true}
}

// FindLatestByBot returns the bot's remembered runs, newest first.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) FindLatestByBot(
	executionContext context.Context, strategyBotID uint,
) ([]entities.StrategyBotRunRecord, error) {
	runRecords := []entities.StrategyBotRunRecord{}

	result := strategyBotRunRecordRepository.database.within(executionContext).
		Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
		Order("run_number DESC").
		Limit(strategyBotRememberedRunCount).
		Find(&runRecords)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy bot run records: %w", result.Error)
	}

	return runRecords, nil
}

// FindByJournalLinkIdentifier answers false once the round has been trimmed away.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) FindByJournalLinkIdentifier(
	executionContext context.Context, journalLinkIdentifier string,
) (entities.StrategyBotRunRecord, bool, error) {
	runRecords := []entities.StrategyBotRunRecord{}
	if journalLinkIdentifier == "" {
		return entities.StrategyBotRunRecord{}, false, nil
	}

	result := strategyBotRunRecordRepository.database.within(executionContext).
		Where(clause.Eq{Column: "journal_link_identifier", Value: journalLinkIdentifier}).
		Limit(1).
		Find(&runRecords)
	if result.Error != nil {
		return entities.StrategyBotRunRecord{}, false, fmt.Errorf("find strategy bot run record by journal link: %w", result.Error)
	}
	if len(runRecords) == 0 {
		return entities.StrategyBotRunRecord{}, false, nil
	}

	return runRecords[0], true, nil
}
