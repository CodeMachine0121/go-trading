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

// StrategyScriptNameIndex is the unique index on an owner and a strategy script's name, and
// uniqueViolationCode is what PostgreSQL calls a broken unique constraint. Together
// they are how a name clash is told apart from any other constraint on the table —
// the primary key included, which breaks when a restored dump leaves the identifier
// sequence behind and has nothing to do with anyone's choice of name.
//
// The index name is repeated from the entity's tag because a struct tag cannot hold
// a constant. If the two ever drift, storing a duplicate name stops being reported
// as a conflict and starts being reported as a storage failure. It is exported so
// that the agreement between the two spellings is asserted by a test needing no
// database, rather than only by one that skips when there is none.
const StrategyScriptNameIndex = "idx_strategies_owner_name"

// uniqueViolationCode is what PostgreSQL calls a broken unique constraint.
const uniqueViolationCode = "23505"

// strategy scriptWritableColumns are the only columns a rewrite may touch. Naming them is
// what makes "the identifier and the time it was first saved never change" true:
// they are not on the list, so no update can reach them however the entity handed in
// was filled.
//
// The owner is not on the list either, which is what makes "a strategy script never
// changes hands" a thing this code cannot express rather than a thing it remembers.
var strategyScriptWritableColumns = []string{
	"name", "description", "script", "result_type",
}

// StrategyScriptRepository stores saved strategy scripts in PostgreSQL.
type StrategyScriptRepository struct {
	database *gorm.DB
}

func NewStrategyScriptRepository(database *gorm.DB) *StrategyScriptRepository {
	return &StrategyScriptRepository{database: database}
}

// Save stores a new strategy script, letting the unique index on the name decide whether it
// may exist. Asking first and creating afterwards would let two requests arriving at
// once both find the name free.
func (strategyScriptRepository *StrategyScriptRepository) Save(
	executionContext context.Context, strategyScript entities.StrategyScript,
) (entities.StrategyScript, error) {
	result := strategyScriptRepository.database.WithContext(executionContext).Create(&strategyScript)
	if writeError := strategyScriptRepository.writeFailureOf(result.Error, strategyScript.Name, "save"); writeError != nil {
		return entities.StrategyScript{}, writeError
	}

	return strategyScript, nil
}

// Update rewrites the five things a strategy script remembers and hands back the strategy script as
// it now stands. Only the writable columns are sent, so the identifier and the time
// the strategy script was first saved are out of reach by construction.
func (strategyScriptRepository *StrategyScriptRepository) Update(
	executionContext context.Context, strategyScript entities.StrategyScript,
) (entities.StrategyScript, error) {
	// The write and the read-back share one transaction, so what comes back is what
	// this call stored. Apart, a second rewrite landing between them would hand this
	// caller somebody else's values as though they were its own — and a deletion
	// landing there would report not found for a row this call had just written.
	updatedStrategyScript := entities.StrategyScript{}

	transactionError := strategyScriptRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The strategy script handed to Model carries the identifier, which is what
			// picks the row; only the writable columns are then sent.
			result := transaction.
				Model(&entities.StrategyScript{ID: strategyScript.ID}).
				Select(strategyScriptWritableColumns).
				Updates(strategyScript)

			if writeError := strategyScriptRepository.writeFailureOf(
				result.Error, strategyScript.Name, "update"); writeError != nil {
				return writeError
			}

			// The knobs are replaced outright rather than reconciled one by one.
			// They have no identity a caller ever names — a knob is its name inside
			// its strategy script — so "which of these is the same knob as before" is a
			// question nobody asks and this one does not answer.
			if parametersError := strategyScriptRepository.replaceParameters(
				transaction, strategyScript); parametersError != nil {
				return parametersError
			}

			// Reading it back is also what reports a strategy script that is not there:
			// nothing was rewritten, so nothing can be found. Checking the rows
			// written first would ask the same question twice.
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

// strategy scriptParametersAssociation is how GORM is asked for a strategy script's knobs. It is
// written once here so that every read fetches them the same way — a strategy script read
// back without them looks like a strategy script that has none.
const strategyScriptParametersAssociation = "Parameters"

// strategy scriptPublicationAssociation is how GORM is asked whether a strategy script is on the
// marketplace. It is read alongside an owner's own strategy scripts because that is the
// only place the answer is used — it decides whether the button in front of them
// publishes or withdraws — and asking per strategy script would be one query each.
const strategyScriptPublicationAssociation = "Publication"

// The three associations a marketplace row is read with. A publication on its own
// is an identifier and a moment; what a reader wants is the strategy script behind it, the
// knobs it declares and who published it.
const (
	publishedStrategyScriptAssociation           = "StrategyScript"
	publishedStrategyScriptParametersAssociation = "StrategyScript.Parameters"
	publishedStrategyScriptOwnerAssociation      = "StrategyScript.Owner"
)

// replaceParameters swaps a strategy script's whole set of knobs for the ones handed in.
//
// Deleting then inserting, rather than working out which rows changed, is the honest
// shape here: a knob has no identity of its own, so there is nothing to match old
// rows against. Both statements share the caller's transaction, so a reader never
// sees a strategy script midway between two sets.
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

// writeFailureOf says what went wrong with a write, in the terms the domain uses. A
// broken name index is the one storage failure that is really a business answer: the
// name belongs to another strategy script. It is written here once because both writes
// reach the same index and owe the caller the same answer.
//
// Every other broken constraint stays a storage failure. Answering "that name is
// taken" for a clash the name had no part in would send whoever reads it hunting for
// a strategy script that does not exist.
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

// isNameAlreadyHeld says whether this write broke the name index specifically.
func (strategyScriptRepository *StrategyScriptRepository) isNameAlreadyHeld(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == StrategyScriptNameIndex
}

// FindOne returns the strategy script carrying this identifier.
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

// FindAllOwnedBy returns every strategy script belonging to this owner, ordered by name.
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
		return nil, fmt.Errorf("find strategy scriptScripts: %w", result.Error)
	}

	return strategyScripts, nil
}

// FindAllPublished returns everything on the marketplace, newest publication first,
// each row already carrying the strategy script, its knobs and its owner.
//
// Reading all four together is one question rather than four: a listing that came
// back as identifiers would send the caller round again per row, and the marketplace
// is the one page where every row needs all of it.
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
		return nil, fmt.Errorf("find published strategy scriptScripts: %w", result.Error)
	}

	return publications, nil
}

// FindAllAdoptedBy returns everything this person has taken off the marketplace and
// that is still on it, ordered by the strategy script's name.
//
// "Still on it" needs no clause of its own: an adoption points at a publication and
// goes with it, so a row that is here has a publication by construction. The join
// onto the adoptions is what narrows this to one person's shelf, and it is also why
// the ordering is stated over the strategy script's own column rather than the row's.
func (strategyScriptRepository *StrategyScriptRepository) FindAllAdoptedBy(
	executionContext context.Context, userID uint,
) ([]entities.PublishedStrategyScript, error) {
	publications := make([]entities.PublishedStrategyScript, 0)

	result := strategyScriptRepository.database.WithContext(executionContext).
		Model(&entities.PublishedStrategyScript{}).
		Joins(`JOIN "StrategyAdoptions" ON "StrategyAdoptions".strategy_id = "PublishedStrategies".strategy_id`).
		Joins(`JOIN "Strategies" ON "Strategies".id = "PublishedStrategies".strategy_id`).
		Where(clause.Eq{Column: clause.Column{Table: "StrategyAdoptions", Name: "user_id"}, Value: userID}).
		Preload(publishedStrategyScriptAssociation).
		Preload(publishedStrategyScriptParametersAssociation).
		Preload(publishedStrategyScriptOwnerAssociation).
		Order(clause.OrderByColumn{Column: clause.Column{Table: "Strategies", Name: "name"}}).
		Find(&publications)
	if result.Error != nil {
		return nil, fmt.Errorf("find adopted strategy scriptScripts: %w", result.Error)
	}

	return publications, nil
}

// Delete removes the strategy script for good. There is no keeping of what was deleted:
// a name that is still held by something nobody can read is a name nobody can
// explain, and this is a single person's own collection.
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
