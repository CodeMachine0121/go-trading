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

// StrategyScriptNameIndex repeats the entity's index tag (tags cannot hold constants); it is exported so a database-free test asserts the two spellings agree.
const StrategyScriptNameIndex = "idx_strategies_owner_name"

const uniqueViolationCode = "23505"

// strategyScriptWritableColumns excludes the identifier, creation time and owner, so no update can change them.
var strategyScriptWritableColumns = []string{
	"name", "description", "script", "result_type",
}

type StrategyScriptRepository struct {
	database *gorm.DB
}

func NewStrategyScriptRepository(database *gorm.DB) *StrategyScriptRepository {
	return &StrategyScriptRepository{database: database}
}

// Save lets the unique name index decide, since check-then-create races.
func (strategyScriptRepository *StrategyScriptRepository) Save(
	executionContext context.Context, strategyScript entities.StrategyScript,
) (entities.StrategyScript, error) {
	result := strategyScriptRepository.database.WithContext(executionContext).Create(&strategyScript)
	if writeError := strategyScriptRepository.writeFailureOf(result.Error, strategyScript.Name, "save"); writeError != nil {
		return entities.StrategyScript{}, writeError
	}

	return strategyScript, nil
}

// Update writes only the writable columns and returns the stored row.
func (strategyScriptRepository *StrategyScriptRepository) Update(
	executionContext context.Context, strategyScript entities.StrategyScript,
) (entities.StrategyScript, error) {
	// Write and read-back share one transaction so a concurrent rewrite or delete cannot leak into the result.
	updatedStrategyScript := entities.StrategyScript{}

	transactionError := strategyScriptRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			result := transaction.
				Model(&entities.StrategyScript{ID: strategyScript.ID}).
				Select(strategyScriptWritableColumns).
				Updates(strategyScript)

			if writeError := strategyScriptRepository.writeFailureOf(
				result.Error, strategyScript.Name, "update"); writeError != nil {
				return writeError
			}

			// Parameters have no identity of their own, so they are replaced outright.
			if parametersError := strategyScriptRepository.replaceParameters(
				transaction, strategyScript); parametersError != nil {
				return parametersError
			}

			// The read-back also reports a missing script as not found.
			readBack := transaction.
				Preload(strategyScriptParametersAssociation).
				First(&updatedStrategyScript, strategyScript.ID)
			if errors.Is(readBack.Error, gorm.ErrRecordNotFound) {
				return domains.StrategyScriptNotFound(strategyScript.ID)
			}
			if readBack.Error != nil {
				return fmt.Errorf("find strategy script: %w", readBack.Error)
			}

			return nil
		})

	if transactionError != nil {
		return entities.StrategyScript{}, transactionError
	}

	return updatedStrategyScript, nil
}

// A script read without this association looks like one with no parameters.
const strategyScriptParametersAssociation = "Parameters"

// Preloaded with an owner's scripts to decide publish vs. withdraw without a query per script.
const strategyScriptPublicationAssociation = "Publication"

const (
	publishedStrategyScriptAssociation           = "StrategyScript"
	publishedStrategyScriptParametersAssociation = "StrategyScript.Parameters"
	publishedStrategyScriptOwnerAssociation      = "StrategyScript.Owner"
)

// replaceParameters deletes then inserts within the caller's transaction, since parameters have no identity to diff against.
func (strategyScriptRepository *StrategyScriptRepository) replaceParameters(
	transaction *gorm.DB, strategyScript entities.StrategyScript,
) error {
	deleted := transaction.
		Where(clause.Eq{Column: "strategy_id", Value: strategyScript.ID}).
		Delete(&entities.StrategyScriptParameter{})
	if deleted.Error != nil {
		return fmt.Errorf("replace strategy script parameters: %w", deleted.Error)
	}

	if len(strategyScript.Parameters) == 0 {
		return nil
	}

	storedParameters := make([]entities.StrategyScriptParameter, 0, len(strategyScript.Parameters))
	for _, parameter := range strategyScript.Parameters {
		parameter.StrategyScriptID = strategyScript.ID
		storedParameters = append(storedParameters, parameter)
	}

	created := transaction.Create(&storedParameters)
	if created.Error != nil {
		return fmt.Errorf("replace strategy script parameters: %w", created.Error)
	}

	return nil
}

// writeFailureOf maps a broken name index to a name conflict; every other constraint violation stays a storage failure.
func (strategyScriptRepository *StrategyScriptRepository) writeFailureOf(
	writeError error, name string, attempt string,
) error {
	if strategyScriptRepository.isNameAlreadyHeld(writeError) {
		return fmt.Errorf("%w: 策略腳本名稱「%s」已被使用", domains.ErrStrategyScriptNameConflict, name)
	}
	if writeError != nil {
		return fmt.Errorf("%s strategy script: %w", attempt, writeError)
	}

	return nil
}

func (strategyScriptRepository *StrategyScriptRepository) isNameAlreadyHeld(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == StrategyScriptNameIndex
}

func (strategyScriptRepository *StrategyScriptRepository) FindOne(executionContext context.Context, id uint) (entities.StrategyScript, error) {
	strategyScript := entities.StrategyScript{}

	result := strategyScriptRepository.database.WithContext(executionContext).
		Preload(strategyScriptParametersAssociation).
		Preload(strategyScriptPublicationAssociation).
		First(&strategyScript, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.StrategyScript{}, domains.StrategyScriptNotFound(id)
	}
	if result.Error != nil {
		return entities.StrategyScript{}, fmt.Errorf("find strategy script: %w", result.Error)
	}

	return strategyScript, nil
}

func (strategyScriptRepository *StrategyScriptRepository) FindAllOwnedBy(
	executionContext context.Context, ownerID uint,
) ([]entities.StrategyScript, error) {
	strategyScripts := make([]entities.StrategyScript, 0)

	result := strategyScriptRepository.database.WithContext(executionContext).
		Preload(strategyScriptParametersAssociation).
		Preload(strategyScriptPublicationAssociation).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "name"}}).
		Find(&strategyScripts)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy scripts: %w", result.Error)
	}

	return strategyScripts, nil
}

// FindAllPublished returns the marketplace newest first, preloading script, parameters and owner.
func (strategyScriptRepository *StrategyScriptRepository) FindAllPublished(
	executionContext context.Context,
) ([]entities.PublishedStrategyScript, error) {
	publications := make([]entities.PublishedStrategyScript, 0)

	result := strategyScriptRepository.database.WithContext(executionContext).
		Preload(publishedStrategyScriptAssociation).
		Preload(publishedStrategyScriptParametersAssociation).
		Preload(publishedStrategyScriptOwnerAssociation).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "published_at"}, Desc: true}).
		Find(&publications)
	if result.Error != nil {
		return nil, fmt.Errorf("find published strategy scripts: %w", result.Error)
	}

	return publications, nil
}

// Delete is a hard delete.
func (strategyScriptRepository *StrategyScriptRepository) Delete(executionContext context.Context, id uint) error {
	result := strategyScriptRepository.database.WithContext(executionContext).Delete(&entities.StrategyScript{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete strategy script: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.StrategyScriptNotFound(id)
	}

	return nil
}
