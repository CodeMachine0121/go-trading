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

// contractTradeRecordColumns are the only row columns a save rewrites, so owner, symbol and direction never change.
var contractTradeRecordColumns = []string{
	"leverage", "status", "planned_stop_loss_price", "planned_take_profit_price", "entry_reason",
	"confidence", "review_went_well", "review_went_wrong", "review_next_time", "execution_score",
	"reviewed_at", "opened_at", "closed_at", "updated_at",
}

var closedContractTradeStatuses = []any{string(vo.ContractTradeStatusClosed), string(vo.ContractTradeStatusReviewed)}

type ContractTradeRecordRepository struct {
	database *gorm.DB
}

func NewContractTradeRecordRepository(database *gorm.DB) *ContractTradeRecordRepository {
	return &ContractTradeRecordRepository{database: database}
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) Create(
	executionContext context.Context, record entities.ContractTradeRecord,
) (entities.ContractTradeRecord, error) {
	createdID := uint(0)

	transactionError := contractTradeRecordRepository.database.WithContext(executionContext).Transaction(
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

			return contractTradeRecordRepository.writeChildren(transaction, row.ID, record)
		})
	if transactionError != nil {
		return entities.ContractTradeRecord{}, contractTradeRecordRepository.writeFailureOf(transactionError)
	}

	return contractTradeRecordRepository.FindOne(executionContext, createdID)
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) Save(
	executionContext context.Context, record entities.ContractTradeRecord,
) (entities.ContractTradeRecord, error) {
	transactionError := contractTradeRecordRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			row := record
			row.Fills = nil
			row.Notes = nil
			row.Tags = nil

			updated := transaction.Model(&entities.ContractTradeRecord{}).
				Where(clause.Eq{Column: "id", Value: record.ID}).
				Where(clause.Eq{Column: "is_deleted", Value: false}).
				Select(contractTradeRecordColumns).
				Updates(&row)
			if updated.Error != nil {
				return updated.Error
			}
			// A trade deleted since it was read must not have its children rewritten.
			if updated.RowsAffected == 0 {
				return domains.ContractTradeNotFound(record.ID)
			}

			survivingFillIDs := []uint{}
			for _, fill := range record.Fills {
				if fill.ID != 0 {
					survivingFillIDs = append(survivingFillIDs, fill.ID)
				}
			}

			removedFills := transaction.Where(clause.Eq{Column: "contract_trade_record_id", Value: record.ID})
			if len(survivingFillIDs) > 0 {
				removedFills = removedFills.Not(map[string]any{"id": survivingFillIDs})
			}
			if deleteError := removedFills.Delete(&entities.ContractTradeFill{}).Error; deleteError != nil {
				return deleteError
			}

			return contractTradeRecordRepository.writeChildren(transaction, record.ID, record)
		})
	if transactionError != nil {
		return entities.ContractTradeRecord{}, contractTradeRecordRepository.writeFailureOf(transactionError)
	}

	return contractTradeRecordRepository.FindOne(executionContext, record.ID)
}

// writeChildren saves new fills and notes, rewrites surviving fills in place, and replaces the tag links.
func (contractTradeRecordRepository *ContractTradeRecordRepository) writeChildren(
	transaction *gorm.DB, recordID uint, record entities.ContractTradeRecord,
) error {
	for _, fill := range record.Fills {
		fill.ContractTradeRecordID = recordID
		if writeError := transaction.Save(&fill).Error; writeError != nil {
			return writeError
		}
	}

	for _, note := range record.Notes {
		if note.ID != 0 {
			continue
		}
		note.ContractTradeRecordID = recordID
		if createError := transaction.Create(&note).Error; createError != nil {
			return createError
		}
	}

	tags := record.Tags
	if tags == nil {
		tags = []entities.TradeTag{}
	}

	return transaction.Model(&entities.ContractTradeRecord{ID: recordID}).
		Omit("Tags.*").
		Association("Tags").
		Replace(tags)
}

// writeFailureOf maps a broken open-position index to its business meaning; anything else stays a fault.
func (contractTradeRecordRepository *ContractTradeRecordRepository) writeFailureOf(writeError error) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == ContractTradeOneOpenPerSymbolDirectionIndex {
		return domains.ContractTradeOpenPositionExists("這個合約標的", "這個方向", 0)
	}

	return fmt.Errorf("save contract trade record: %w", writeError)
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.ContractTradeRecord, error) {
	record := entities.ContractTradeRecord{}

	result := contractTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ContractTradeRecord{}, domains.ContractTradeNotFound(id)
	}
	if result.Error != nil {
		return entities.ContractTradeRecord{}, fmt.Errorf("find contract trade record: %w", result.Error)
	}

	return record, nil
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) FindPageByOwner(
	executionContext context.Context, ownerID uint, filter vo.TradeListFilterVo,
) ([]entities.ContractTradeRecord, int64, error) {
	matching := contractTradeRecordRepository.notDeleted(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID})
	if filter.Status != "" {
		matching = matching.Where(clause.Eq{Column: "status", Value: filter.Status})
	}
	if filter.Symbol != "" {
		matching = matching.Where(clause.Eq{Column: "symbol", Value: filter.Symbol})
	}
	if filter.OpenedSince != nil {
		matching = matching.Where(clause.Gte{Column: "opened_at", Value: filter.OpenedSince.UTC()})
	}

	totalCount := int64(0)
	if countError := matching.Session(&gorm.Session{}).Count(&totalCount).Error; countError != nil {
		return nil, 0, fmt.Errorf("count contract trade records: %w", countError)
	}

	records := []entities.ContractTradeRecord{}
	result := contractTradeRecordRepository.preloadChildren(matching.Session(&gorm.Session{})).
		Order("opened_at DESC").Order("id DESC").
		Limit(filter.Limit).
		Find(&records)
	if result.Error != nil {
		return nil, 0, fmt.Errorf("list contract trade records: %w", result.Error)
	}

	return records, totalCount, nil
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) FindClosedByOwner(
	executionContext context.Context, ownerID uint, closedSince *time.Time,
) ([]entities.ContractTradeRecord, error) {
	query := contractTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.IN{Column: clause.Column{Name: "status"}, Values: closedContractTradeStatuses})
	if closedSince != nil {
		query = query.Where(clause.Gte{Column: "closed_at", Value: closedSince.UTC()})
	}

	records := []entities.ContractTradeRecord{}
	if findError := query.Order("closed_at ASC").Order("id ASC").Find(&records).Error; findError != nil {
		return nil, fmt.Errorf("list closed contract trade records: %w", findError)
	}

	return records, nil
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) FindClosedByOwnerAndTradingStrategy(
	executionContext context.Context, ownerID uint, tradingStrategyID uint,
) ([]entities.ContractTradeRecord, error) {
	records := []entities.ContractTradeRecord{}

	result := contractTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyID}).
		Where(clause.IN{Column: clause.Column{Name: "status"}, Values: closedContractTradeStatuses}).
		Order("closed_at ASC").Order("id ASC").
		Find(&records)
	if result.Error != nil {
		return nil, fmt.Errorf("list contract trade records of a trading strategy: %w", result.Error)
	}

	return records, nil
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) FindOpenByOwnerSymbolDirection(
	executionContext context.Context, ownerID uint, symbol string, direction string,
) (entities.ContractTradeRecord, bool, error) {
	records := []entities.ContractTradeRecord{}

	result := contractTradeRecordRepository.withChildren(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Eq{Column: "direction", Value: direction}).
		Where(clause.Eq{Column: "status", Value: string(vo.ContractTradeStatusOpen)}).
		Limit(1).
		Find(&records)
	if result.Error != nil {
		return entities.ContractTradeRecord{}, false, fmt.Errorf("find open contract trade record: %w", result.Error)
	}
	if len(records) == 0 {
		return entities.ContractTradeRecord{}, false, nil
	}

	return records[0], true, nil
}

// MarkDeleted only touches a trade not yet deleted, so a second delete neither succeeds nor moves the deletion time.
func (contractTradeRecordRepository *ContractTradeRecordRepository) MarkDeleted(
	executionContext context.Context, id uint, deletedAt time.Time,
) error {
	result := contractTradeRecordRepository.notDeleted(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Updates(map[string]any{"is_deleted": true, "deleted_at": deletedAt.UTC()})
	if result.Error != nil {
		return fmt.Errorf("mark contract trade record deleted: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.ContractTradeNotFound(id)
	}

	return nil
}

// CountByTag counts only trades not deleted, reading the tag links first so no join has to be spelled out.
func (contractTradeRecordRepository *ContractTradeRecordRepository) CountByTag(
	executionContext context.Context, tagID uint,
) (int64, error) {
	taggedRecordIDs := []uint{}
	linkResult := contractTradeRecordRepository.database.WithContext(executionContext).
		Table("contract_trade_record_tags").
		Where(clause.Eq{Column: "trade_tag_id", Value: tagID}).
		Pluck("contract_trade_record_id", &taggedRecordIDs)
	if linkResult.Error != nil {
		return 0, fmt.Errorf("find trades carrying a tag: %w", linkResult.Error)
	}
	if len(taggedRecordIDs) == 0 {
		return 0, nil
	}

	taggedRecordIDValues := make([]any, 0, len(taggedRecordIDs))
	for _, taggedRecordID := range taggedRecordIDs {
		taggedRecordIDValues = append(taggedRecordIDValues, taggedRecordID)
	}

	count := int64(0)
	countResult := contractTradeRecordRepository.notDeleted(executionContext).
		Where(clause.IN{Column: clause.Column{Name: "id"}, Values: taggedRecordIDValues}).
		Count(&count)
	if countResult.Error != nil {
		return 0, fmt.Errorf("count trades carrying a tag: %w", countResult.Error)
	}

	return count, nil
}

// notDeleted is where every read starts, so a deleted trade is invisible without each query remembering it.
func (contractTradeRecordRepository *ContractTradeRecordRepository) notDeleted(
	executionContext context.Context,
) *gorm.DB {
	return contractTradeRecordRepository.database.WithContext(executionContext).
		Model(&entities.ContractTradeRecord{}).
		Where(clause.Eq{Column: "is_deleted", Value: false})
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) withChildren(
	executionContext context.Context,
) *gorm.DB {
	return contractTradeRecordRepository.preloadChildren(
		contractTradeRecordRepository.notDeleted(executionContext))
}

func (contractTradeRecordRepository *ContractTradeRecordRepository) preloadChildren(query *gorm.DB) *gorm.DB {
	return query.
		Preload("Fills", func(fills *gorm.DB) *gorm.DB { return fills.Order("filled_at ASC").Order("id ASC") }).
		Preload("Notes", func(notes *gorm.DB) *gorm.DB { return notes.Order("created_at ASC").Order("id ASC") }).
		Preload("Tags", func(tags *gorm.DB) *gorm.DB { return tags.Order("kind ASC").Order("name ASC") })
}
