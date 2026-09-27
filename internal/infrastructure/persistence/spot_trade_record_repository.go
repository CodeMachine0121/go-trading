package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// spotTradeRecordColumns are the only row columns a save rewrites, so owner, symbol and market never change.
var spotTradeRecordColumns = []string{
	"status", "planned_stop_loss_price", "planned_take_profit_price", "entry_reason",
	"confidence", "review_went_well", "review_went_wrong", "review_next_time", "execution_score",
	"reviewed_at", "opened_at", "closed_at", "updated_at",
}

var closedSpotTradeStatuses = []any{string(vo.SpotTradeStatusClosed), string(vo.SpotTradeStatusReviewed)}

type SpotTradeRecordRepository struct {
	database *gorm.DB
}

func NewSpotTradeRecordRepository(database *gorm.DB) *SpotTradeRecordRepository {
	return &SpotTradeRecordRepository{database: database}
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) Create(
	executionContext context.Context, record entities.SpotTradeRecord,
) (entities.SpotTradeRecord, error) {
	createdID := uint(0)

	transactionError := spotTradeRecordRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			row := record
			row.ID = 0
			row.Fills = nil
			row.Notes = nil
			row.Tags = nil

			if createError := transaction.Omit(clause.Associations).Create(&row).Error; createError != nil {
				return createError
			}
			createdID = row.ID

			return spotTradeRecordRepository.writeChildren(transaction, row.ID, record)
		})
	if transactionError != nil {
		return entities.SpotTradeRecord{}, spotTradeRecordRepository.writeFailureOf(transactionError)
	}

	return spotTradeRecordRepository.FindOne(executionContext, createdID)
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) Save(
	executionContext context.Context, record entities.SpotTradeRecord,
) (entities.SpotTradeRecord, error) {
	transactionError := spotTradeRecordRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			row := record
			row.Fills = nil
			row.Notes = nil
			row.Tags = nil

			if updateError := transaction.Model(&entities.SpotTradeRecord{}).
				Where(clause.Eq{Column: "id", Value: record.ID}).
				Select(spotTradeRecordColumns).
				Updates(&row).Error; updateError != nil {
				return updateError
			}

			survivingFillIDs := []uint{}
			for _, fill := range record.Fills {
				if fill.ID != 0 {
					survivingFillIDs = append(survivingFillIDs, fill.ID)
				}
			}

			removedFills := transaction.Where(clause.Eq{Column: "spot_trade_record_id", Value: record.ID})
			if len(survivingFillIDs) > 0 {
				removedFills = removedFills.Not(map[string]any{"id": survivingFillIDs})
			}
			if deleteError := removedFills.Delete(&entities.SpotTradeFill{}).Error; deleteError != nil {
				return deleteError
			}

			return spotTradeRecordRepository.writeChildren(transaction, record.ID, record)
		})
	if transactionError != nil {
		return entities.SpotTradeRecord{}, spotTradeRecordRepository.writeFailureOf(transactionError)
	}

	return spotTradeRecordRepository.FindOne(executionContext, record.ID)
}

// writeChildren saves new fills and notes, rewrites surviving fills in place, and replaces the tag links.
func (spotTradeRecordRepository *SpotTradeRecordRepository) writeChildren(
	transaction *gorm.DB, recordID uint, record entities.SpotTradeRecord,
) error {
	for _, fill := range record.Fills {
		fill.SpotTradeRecordID = recordID
		if writeError := transaction.Save(&fill).Error; writeError != nil {
			return writeError
		}
	}

	for _, note := range record.Notes {
		if note.ID != 0 {
			continue
		}
		note.SpotTradeRecordID = recordID
		if createError := transaction.Create(&note).Error; createError != nil {
			return createError
		}
	}

	tags := record.Tags
	if tags == nil {
		tags = []entities.TradeTag{}
	}

	return transaction.Model(&entities.SpotTradeRecord{ID: recordID}).
		Omit("Tags.*").
		Association("Tags").
		Replace(tags)
}

// writeFailureOf maps a broken open-holding index to its business meaning; anything else stays a fault.
func (spotTradeRecordRepository *SpotTradeRecordRepository) writeFailureOf(writeError error) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == SpotTradeOneOpenPerSymbolIndex {
		return domains.SpotTradeOpenHoldingExists("這個標的", 0)
	}

	return fmt.Errorf("save spot trade record: %w", writeError)
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.SpotTradeRecord, error) {
	record := entities.SpotTradeRecord{}

	result := spotTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.SpotTradeRecord{}, domains.SpotTradeNotFound(id)
	}
	if result.Error != nil {
		return entities.SpotTradeRecord{}, fmt.Errorf("find spot trade record: %w", result.Error)
	}

	return record, nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) FindPageByOwner(
	executionContext context.Context, ownerID uint, filter vo.TradeListFilterVo,
) ([]entities.SpotTradeRecord, int64, error) {
	matching := spotTradeRecordRepository.database.WithContext(executionContext).
		Model(&entities.SpotTradeRecord{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID})
	if filter.Status != "" {
		matching = matching.Where(clause.Eq{Column: "status", Value: filter.Status})
	}
	if filter.Symbol != "" {
		matching = matching.Where(clause.Eq{Column: "symbol", Value: filter.Symbol})
	}
	if filter.Market != "" {
		matching = matching.Where(clause.Eq{Column: "market", Value: filter.Market})
	}
	if filter.OpenedSince != nil {
		matching = matching.Where(clause.Gte{Column: "opened_at", Value: filter.OpenedSince.UTC()})
	}

	totalCount := int64(0)
	if countError := matching.Session(&gorm.Session{}).Count(&totalCount).Error; countError != nil {
		return nil, 0, fmt.Errorf("count spot trade records: %w", countError)
	}

	records := []entities.SpotTradeRecord{}
	result := spotTradeRecordRepository.preloadChildren(matching.Session(&gorm.Session{})).
		Order("opened_at DESC").Order("id DESC").
		Limit(filter.Limit).
		Find(&records)
	if result.Error != nil {
		return nil, 0, fmt.Errorf("list spot trade records: %w", result.Error)
	}

	return records, totalCount, nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) FindClosedByOwner(
	executionContext context.Context, ownerID uint, closedSince *time.Time,
) ([]entities.SpotTradeRecord, error) {
	query := spotTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.IN{Column: clause.Column{Name: "status"}, Values: closedSpotTradeStatuses})
	if closedSince != nil {
		query = query.Where(clause.Gte{Column: "closed_at", Value: closedSince.UTC()})
	}

	records := []entities.SpotTradeRecord{}
	if findError := query.Order("closed_at ASC").Order("id ASC").Find(&records).Error; findError != nil {
		return nil, fmt.Errorf("list closed spot trade records: %w", findError)
	}

	return records, nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) FindClosedByOwnerAndTradingStrategy(
	executionContext context.Context, ownerID uint, tradingStrategyID uint,
) ([]entities.SpotTradeRecord, error) {
	records := []entities.SpotTradeRecord{}

	result := spotTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyID}).
		Where(clause.IN{Column: clause.Column{Name: "status"}, Values: closedSpotTradeStatuses}).
		Order("closed_at ASC").Order("id ASC").
		Find(&records)
	if result.Error != nil {
		return nil, fmt.Errorf("list spot trade records of a trading strategy: %w", result.Error)
	}

	return records, nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) FindOpenByOwnerSymbol(
	executionContext context.Context, ownerID uint, symbol string,
) (entities.SpotTradeRecord, bool, error) {
	records := []entities.SpotTradeRecord{}

	result := spotTradeRecordRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Eq{Column: "status", Value: string(vo.SpotTradeStatusOpen)}).
		Limit(1).
		Find(&records)
	if result.Error != nil {
		return entities.SpotTradeRecord{}, false, fmt.Errorf("find open spot trade record: %w", result.Error)
	}
	if len(records) == 0 {
		return entities.SpotTradeRecord{}, false, nil
	}

	return records[0], true, nil
}

// Delete removes the trade; its fills, notes and tag links cascade.
func (spotTradeRecordRepository *SpotTradeRecordRepository) Delete(
	executionContext context.Context, id uint,
) error {
	result := spotTradeRecordRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Delete(&entities.SpotTradeRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete spot trade record: %w", result.Error)
	}

	return nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) CountByTag(
	executionContext context.Context, tagID uint,
) (int64, error) {
	count := int64(0)

	result := spotTradeRecordRepository.database.WithContext(executionContext).
		Table("spot_trade_record_tags").
		Where(clause.Eq{Column: "trade_tag_id", Value: tagID}).
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count trades carrying a tag: %w", result.Error)
	}

	return count, nil
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) withChildren(
	executionContext context.Context,
) *gorm.DB {
	return spotTradeRecordRepository.preloadChildren(
		spotTradeRecordRepository.database.WithContext(executionContext))
}

func (spotTradeRecordRepository *SpotTradeRecordRepository) preloadChildren(query *gorm.DB) *gorm.DB {
	return query.
		Preload("Fills", func(fills *gorm.DB) *gorm.DB { return fills.Order("filled_at ASC").Order("id ASC") }).
		Preload("Notes", func(notes *gorm.DB) *gorm.DB { return notes.Order("created_at ASC").Order("id ASC") }).
		Preload("Tags", func(tags *gorm.DB) *gorm.DB { return tags.Order("kind ASC").Order("name ASC") })
}
