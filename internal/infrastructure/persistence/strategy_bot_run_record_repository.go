package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// strategyBotRememberedRunCount caps each watched symbol's run history, trimmed on write so no separate cleanup job is needed; a bot-wide cap would leave a ten-symbol bot five rounds per symbol.
const strategyBotRememberedRunCount = 50

// It reads through ambientTransactionDatabase so a booked round can land in one transaction with what it caused.
type StrategyBotRunRecordRepository struct {
	database ambientTransactionDatabase
}

func NewStrategyBotRunRecordRepository(database *gorm.DB) *StrategyBotRunRecordRepository {
	return &StrategyBotRunRecordRepository{database: ambientTransactionDatabase{root: database}}
}

// Append inserts the run and trims its symbol's history in one transaction, so a failed trim cannot let one symbol's history grow unbounded.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) Append(
	executionContext context.Context, writeDto dto.StrategyBotRunRecordWriteDto,
) (int, error) {
	strategyBotID := writeDto.StrategyBotID
	runNumber := 0

	transactionError := strategyBotRunRecordRepository.database.within(executionContext).
		Transaction(func(transaction *gorm.DB) error {
			latestNumber := 0

			// The next run number is read from the bot's latest rather than counted, so trimming never renumbers runs and numbers stay unique across symbols; safe because a bot books one round at a time.
			if selectError := transaction.
				Model(&entities.StrategyBotRunRecord{}).
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Select("COALESCE(MAX(run_number), 0)").
				Scan(&latestNumber).Error; selectError != nil {
				return selectError
			}

			runRecord := entities.StrategyBotRunRecord{
				StrategyBotID:         strategyBotID,
				Symbol:                writeDto.Symbol,
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
			runNumber = runRecord.RunNumber

			// The symbol's oldest remembered number marks the cut, since the bot's other symbols take numbers in between.
			oldestRememberedNumbers := []int{}
			if pluckError := transaction.
				Model(&entities.StrategyBotRunRecord{}).
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Where(clause.Eq{Column: "symbol", Value: writeDto.Symbol}).
				Order("run_number DESC").
				Offset(strategyBotRememberedRunCount-1).
				Limit(1).
				Pluck("run_number", &oldestRememberedNumbers).Error; pluckError != nil {
				return pluckError
			}
			if len(oldestRememberedNumbers) == 0 {
				return nil
			}

			return transaction.
				Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
				Where(clause.Eq{Column: "symbol", Value: writeDto.Symbol}).
				Where(clause.Lt{Column: "run_number", Value: oldestRememberedNumbers[0]}).
				Delete(&entities.StrategyBotRunRecord{}).Error
		})
	if transactionError != nil {
		return 0, fmt.Errorf("append strategy bot run record: %w", transactionError)
	}

	return runNumber, nil
}

// storedFigure marks a suggested figure as present, even if zero.
func storedFigure(figure decimal.Decimal) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: figure, Valid: true}
}

// FindLatestByBot returns the bot's remembered runs, newest first, narrowed to one symbol when one is given; the read is capped at what a full bot remembers, since symbols removed from a bot keep their last rounds.
func (strategyBotRunRecordRepository *StrategyBotRunRecordRepository) FindLatestByBot(
	executionContext context.Context, strategyBotID uint, symbol string,
) ([]entities.StrategyBotRunRecord, error) {
	runRecords := []entities.StrategyBotRunRecord{}

	query := strategyBotRunRecordRepository.database.within(executionContext).
		Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID})
	if symbol != "" {
		query = query.Where(clause.Eq{Column: "symbol", Value: symbol})
	}

	result := query.
		Order("run_number DESC").
		Limit(strategyBotRememberedRunCount * domains.StrategyBotWatchedSymbolLimit).
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
