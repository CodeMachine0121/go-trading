package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SymbolScreeningProfileRepository struct {
	database ambientTransactionDatabase
}

func NewSymbolScreeningProfileRepository(database *gorm.DB) *SymbolScreeningProfileRepository {
	return &SymbolScreeningProfileRepository{database: ambientTransactionDatabase{root: database}}
}

func (profileRepository *SymbolScreeningProfileRepository) Save(
	executionContext context.Context, profile entities.SymbolScreeningProfile,
) (entities.SymbolScreeningProfile, error) {
	saved := profileRepository.database.within(executionContext).Omit("Owner").Save(&profile)
	if saved.Error != nil {
		return entities.SymbolScreeningProfile{}, fmt.Errorf("save symbol screening profile: %w", saved.Error)
	}

	return profile, nil
}

func (profileRepository *SymbolScreeningProfileRepository) FindOneOwned(
	executionContext context.Context, ownerID uint, id uint,
) (entities.SymbolScreeningProfile, bool, error) {
	profile := entities.SymbolScreeningProfile{}
	found := profileRepository.database.within(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		First(&profile)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.SymbolScreeningProfile{}, false, nil
	}
	if found.Error != nil {
		return entities.SymbolScreeningProfile{}, false, fmt.Errorf("find symbol screening profile: %w", found.Error)
	}

	return profile, true, nil
}

func (profileRepository *SymbolScreeningProfileRepository) FindAllByOwner(
	executionContext context.Context, ownerID uint,
) ([]entities.SymbolScreeningProfile, error) {
	profiles := []entities.SymbolScreeningProfile{}
	listed := profileRepository.database.within(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("id ASC").
		Find(&profiles)
	if listed.Error != nil {
		return nil, fmt.Errorf("list symbol screening profiles: %w", listed.Error)
	}

	return profiles, nil
}

func (profileRepository *SymbolScreeningProfileRepository) ExistsName(
	executionContext context.Context, ownerID uint, name string, exceptID uint,
) (bool, error) {
	matchingCount := int64(0)
	counted := profileRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningProfile{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "name", Value: name}).
		Where(clause.Neq{Column: "id", Value: exceptID}).
		Count(&matchingCount)
	if counted.Error != nil {
		return false, fmt.Errorf("count symbol screening profile names: %w", counted.Error)
	}

	return matchingCount > 0, nil
}

func (profileRepository *SymbolScreeningProfileRepository) DeleteOwned(
	executionContext context.Context, ownerID uint, id uint,
) error {
	deleted := profileRepository.database.within(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Delete(&entities.SymbolScreeningProfile{})
	if deleted.Error != nil {
		return fmt.Errorf("delete symbol screening profile: %w", deleted.Error)
	}

	return nil
}
