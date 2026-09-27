package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const TradeTagOwnerKindNameIndex = "idx_trade_tags_owner_kind_name"

type TradeTagRepository struct {
	database *gorm.DB
}

func NewTradeTagRepository(database *gorm.DB) *TradeTagRepository {
	return &TradeTagRepository{database: database}
}

func (tradeTagRepository *TradeTagRepository) FindAllByOwner(
	executionContext context.Context, ownerID uint,
) ([]entities.TradeTag, error) {
	tags := []entities.TradeTag{}

	result := tradeTagRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("kind ASC").Order("name ASC").
		Find(&tags)
	if result.Error != nil {
		return nil, fmt.Errorf("list trade tags: %w", result.Error)
	}

	return tags, nil
}

func (tradeTagRepository *TradeTagRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.TradeTag, error) {
	tag := entities.TradeTag{}

	result := tradeTagRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&tag)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.TradeTag{}, domains.TradeTagNotFound(id)
	}
	if result.Error != nil {
		return entities.TradeTag{}, fmt.Errorf("find trade tag: %w", result.Error)
	}

	return tag, nil
}

func (tradeTagRepository *TradeTagRepository) FindByIDs(
	executionContext context.Context, ids []uint,
) ([]entities.TradeTag, error) {
	tags := []entities.TradeTag{}
	if len(ids) == 0 {
		return tags, nil
	}

	result := tradeTagRepository.database.WithContext(executionContext).Find(&tags, ids)
	if result.Error != nil {
		return nil, fmt.Errorf("find trade tags: %w", result.Error)
	}

	return tags, nil
}

func (tradeTagRepository *TradeTagRepository) Create(
	executionContext context.Context, tag entities.TradeTag,
) (entities.TradeTag, error) {
	tag.ID = 0

	if createError := tradeTagRepository.database.WithContext(executionContext).
		Omit(clause.Associations).Create(&tag).Error; createError != nil {
		return entities.TradeTag{}, tradeTagRepository.writeFailureOf(createError, tag.Name)
	}

	return tag, nil
}

func (tradeTagRepository *TradeTagRepository) CreateIfAbsent(
	executionContext context.Context, tags []entities.TradeTag,
) error {
	if len(tags) == 0 {
		return nil
	}

	result := tradeTagRepository.database.WithContext(executionContext).
		Omit(clause.Associations).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&tags)
	if result.Error != nil {
		return fmt.Errorf("seed trade tags: %w", result.Error)
	}

	return nil
}

func (tradeTagRepository *TradeTagRepository) Rename(
	executionContext context.Context, id uint, name string,
) (entities.TradeTag, error) {
	result := tradeTagRepository.database.WithContext(executionContext).
		Model(&entities.TradeTag{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Update("name", name)
	if result.Error != nil {
		return entities.TradeTag{}, tradeTagRepository.writeFailureOf(result.Error, name)
	}

	return tradeTagRepository.FindOne(executionContext, id)
}

func (tradeTagRepository *TradeTagRepository) Delete(executionContext context.Context, id uint) error {
	result := tradeTagRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Delete(&entities.TradeTag{})
	if result.Error != nil {
		return fmt.Errorf("delete trade tag: %w", result.Error)
	}

	return nil
}

// writeFailureOf maps a broken name index to a name conflict; anything else stays a fault.
func (tradeTagRepository *TradeTagRepository) writeFailureOf(writeError error, name string) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == TradeTagOwnerKindNameIndex {
		return fmt.Errorf("%w: 已有同名的標籤「%s」", domains.ErrTradeTagNameConflict, name)
	}

	return fmt.Errorf("save trade tag: %w", writeError)
}
