package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyScriptService is the application layer's only entry point for saved strategy scripts.
// Its public use-case methods never call one another.
//
// It is given no way to reach a K candle, and that is the point: saving a strategy script
// must not read the market or work anything out, and a dependency that is not there
// cannot be used by accident later.
//
// It does know about the marketplace, but only to ask it one question: is this
// strategy script published? That is the third of the three gates, and every method here
// that hands a strategy script to somebody has to walk them.
type StrategyScriptService struct {
	strategyScriptRepository          domaininterface.IStrategyScriptRepository
	publishedStrategyScriptRepository domaininterface.IPublishedStrategyScriptRepository
}

func NewStrategyScriptService(
	strategyScriptRepository domaininterface.IStrategyScriptRepository,
	publishedStrategyScriptRepository domaininterface.IPublishedStrategyScriptRepository,
) *StrategyScriptService {
	return &StrategyScriptService{
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
	}
}

// CreateStrategyScript saves a new strategy script for its owner and hands it back as stored. A
// strategy script that breaks a rule — including having no owner — is refused before
// anything is written.
func (strategyScriptService *StrategyScriptService) CreateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)
	if validationError != nil {
		return dto.StrategyScriptDto{}, validationError
	}

	savedStrategyScript, saveError := strategyScriptService.strategyScriptRepository.Save(executionContext, strategyScriptDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyScriptDto{}, saveError
	}

	return savedStrategyScript.ToDto(), nil
}

// GetStrategyScript returns the viewer's own strategy script carrying this identifier, script
// and all.
//
// It serves owners only. Somebody else's published strategy script is read from the
// marketplace instead, and that is not an inconvenience worth smoothing over: the
// two hand back different shapes, so they are two questions, and a method that
// answered both would need a shape with a script that is sometimes there — the one
// thing this feature must not have.
func (strategyScriptService *StrategyScriptService) GetStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyScriptDto, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyScriptDto{}, findError
	}

	// Whether it is published is not asked, because it cannot change the answer:
	// this door only opens for the owner.
	return domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false).ToOwnerDto()
}

// ListAvailableStrategyScripts returns what this person picks from day to day: their own
// strategy scripts, and the ones they took off the marketplace.
//
// The two arrive as two lists rather than one. Adopted strategy scripts come back without
// their scripts, and there is no single type that could hold both — which is
// exactly why they are not merged.
func (strategyScriptService *StrategyScriptService) ListAvailableStrategyScripts(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategyScriptsDto, error) {
	ownStrategyScripts, ownError := strategyScriptService.strategyScriptRepository.FindAllOwnedBy(executionContext, viewerID)
	if ownError != nil {
		return dto.AvailableStrategyScriptsDto{}, ownError
	}

	adoptedPublications, adoptedError := strategyScriptService.strategyScriptRepository.FindAllAdoptedBy(
		executionContext, viewerID)
	if adoptedError != nil {
		return dto.AvailableStrategyScriptsDto{}, adoptedError
	}

	mine := make([]dto.StrategyScriptDto, 0, len(ownStrategyScripts))
	for _, strategyScript := range ownStrategyScripts {
		mine = append(mine, strategyScript.ToDto())
	}

	adopted := make([]dto.PublishedStrategyScriptDto, 0, len(adoptedPublications))
	for _, publication := range adoptedPublications {
		adopted = append(adopted, publication.ToDto())
	}

	return dto.AvailableStrategyScriptsDto{Mine: mine, Adopted: adopted}, nil
}

// UpdateStrategyScript rewrites the strategy script this write names and hands it back as it now
// stands. Every rule that governs a new strategy script governs a rewritten one, because
// both arrive here as the same shape and are judged by the same model.
//
// A strategy script that is out on the marketplace is rewritten like any other. Publishing
// hands out the use of an algorithm, not a frozen copy of it: an owner who fixes a
// mistake should not also have to remember to publish again, and whoever is using
// it should get the fix.
func (strategyScriptService *StrategyScriptService) UpdateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	// No strategy script carries no identifier, so there is nothing here to rewrite. Saying
	// so here rather than letting the write go out is the difference between a
	// guarantee this code makes and one it borrows: a rewrite with no identifier
	// names no row, and what an ORM does with a write that names no row is its own
	// decision to change.
	if writeDto.ID == 0 {
		return dto.StrategyScriptDto{}, domains.StrategyScriptNotFound(writeDto.ID)
	}

	// Whether the strategy script is there, and whether it is this caller's, are both
	// settled before its content is judged. The other way round, rewriting somebody
	// else's strategy script with content that is also wrong answers "a strategy script must carry
	// a name" — which tells a stranger their target exists and what is wrong with
	// what they sent.
	if ownershipError := strategyScriptService.requireOwnership(
		executionContext, writeDto.OwnerID, writeDto.ID); ownershipError != nil {
		return dto.StrategyScriptDto{}, ownershipError
	}

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)
	if validationError != nil {
		return dto.StrategyScriptDto{}, validationError
	}

	updatedStrategyScript, updateError := strategyScriptService.strategyScriptRepository.Update(
		executionContext, strategyScriptDomain.ToEntity())
	if updateError != nil {
		return dto.StrategyScriptDto{}, updateError
	}

	return updatedStrategyScript.ToDto(), nil
}

// DeleteStrategyScript removes the viewer's own strategy script for good, taking its place on
// the marketplace and everybody's adoption of it along with it.
func (strategyScriptService *StrategyScriptService) DeleteStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if ownershipError := strategyScriptService.requireOwnership(
		executionContext, viewerID, id); ownershipError != nil {
		return ownershipError
	}

	return strategyScriptService.strategyScriptRepository.Delete(executionContext, id)
}

// ResolveRunnableStrategyScript turns an identifier into the algorithm, the knobs and the
// kind of value behind it, for whoever is allowed to run it.
//
// This is where the three gates are walked in full, and the only way a script
// leaves storage for a run. What comes back goes to the application layer and stops
// there, so running somebody else's strategy script never becomes a way to read it.
//
// Whether the caller has adopted it is not asked. Adoption fills a picker; it does
// not grant anything, or choosing from the marketplace would mean choosing blind.
func (strategyScriptService *StrategyScriptService) ResolveRunnableStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.RunnableStrategyScriptDto{}, findError
	}

	// The third gate is only asked when the second one did not already open. The
	// model says so itself — being runnable is "mine, or published" — so for the
	// owner's own strategy script the marketplace cannot change the answer, and reading it
	// is a query that buys nothing. Reading a strategy script already works this way
	// (see GetStrategyScript); running it now does too.
	//
	// It is not a micro-optimisation. A standing bot resolves every one of its
	// signal sources on every round, for ever, and those are almost always its
	// owner's own strategy scripts — so this is one wasted query per source per round,
	// each of which also logged a "record not found" that meant nothing.
	access := domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false)
	if access.IsOwnedByViewer() {
		return access.ToRunnableDto()
	}

	isPublished, publicationError := strategyScriptService.isPublished(executionContext, id)
	if publicationError != nil {
		return dto.RunnableStrategyScriptDto{}, publicationError
	}

	return domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, isPublished).ToRunnableDto()
}

// requireOwnership is the two steps that stand in front of changing a strategy script:
// find it, then ask whether it is this caller's. Rewriting and deleting both need
// them, and both owe a stranger the same sentence as a strategy script that is not there.
//
// It stops at the second gate on purpose. Publishing hands out the use of an
// algorithm, never the right to alter it, so whether the strategy script is on the
// marketplace cannot change this answer — and not asking saves a read.
func (strategyScriptService *StrategyScriptService) requireOwnership(
	executionContext context.Context, viewerID uint, id uint,
) error {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return findError
	}

	return domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false).RequireOwnership()
}

// isPublished answers the third gate. "There is no publication" is not a failure to
// report upwards — it is one of the two answers — so it is read here and turned
// into a plain no.
func (strategyScriptService *StrategyScriptService) isPublished(
	executionContext context.Context, strategyScriptID uint,
) (bool, error) {
	_, findError := strategyScriptService.publishedStrategyScriptRepository.FindOne(executionContext, strategyScriptID)
	if errors.Is(findError, domains.ErrStrategyScriptNotPublished) {
		return false, nil
	}
	if findError != nil {
		return false, findError
	}

	return true, nil
}
