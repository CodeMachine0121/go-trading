package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyService is the application layer's only entry point for saved strategies.
// Its public use-case methods never call one another.
//
// It is given no way to reach a K candle, and that is the point: saving a strategy
// must not read the market or work anything out, and a dependency that is not there
// cannot be used by accident later.
//
// It does know about the marketplace, but only to ask it one question: is this
// strategy published? That is the third of the three gates, and every method here
// that hands a strategy to somebody has to walk them.
type StrategyService struct {
	strategyRepository          domaininterface.IStrategyRepository
	publishedStrategyRepository domaininterface.IPublishedStrategyRepository
}

func NewStrategyService(
	strategyRepository domaininterface.IStrategyRepository,
	publishedStrategyRepository domaininterface.IPublishedStrategyRepository,
) *StrategyService {
	return &StrategyService{
		strategyRepository:          strategyRepository,
		publishedStrategyRepository: publishedStrategyRepository,
	}
}

// CreateStrategy saves a new strategy for its owner and hands it back as stored. A
// strategy that breaks a rule — including having no owner — is refused before
// anything is written.
func (strategyService *StrategyService) CreateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	savedStrategy, saveError := strategyService.strategyRepository.Save(executionContext, strategyDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyDto{}, saveError
	}

	return savedStrategy.ToDto(), nil
}

// GetStrategy returns the viewer's own strategy carrying this identifier, script
// and all.
//
// It serves owners only. Somebody else's published strategy is read from the
// marketplace instead, and that is not an inconvenience worth smoothing over: the
// two hand back different shapes, so they are two questions, and a method that
// answered both would need a shape with a script that is sometimes there — the one
// thing this feature must not have.
func (strategyService *StrategyService) GetStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyDto, error) {
	strategy, findError := strategyService.strategyRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyDto{}, findError
	}

	// Whether it is published is not asked, because it cannot change the answer:
	// this door only opens for the owner.
	return domains.NewStrategyAccessDomain(strategy, viewerID, false).ToOwnerDto()
}

// ListAvailableStrategies returns what this person picks from day to day: their own
// strategies, and the ones they took off the marketplace.
//
// The two arrive as two lists rather than one. Adopted strategies come back without
// their scripts, and there is no single type that could hold both — which is
// exactly why they are not merged.
func (strategyService *StrategyService) ListAvailableStrategies(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategiesDto, error) {
	ownStrategies, ownError := strategyService.strategyRepository.FindAllOwnedBy(executionContext, viewerID)
	if ownError != nil {
		return dto.AvailableStrategiesDto{}, ownError
	}

	adoptedPublications, adoptedError := strategyService.strategyRepository.FindAllAdoptedBy(
		executionContext, viewerID)
	if adoptedError != nil {
		return dto.AvailableStrategiesDto{}, adoptedError
	}

	mine := make([]dto.StrategyDto, 0, len(ownStrategies))
	for _, strategy := range ownStrategies {
		mine = append(mine, strategy.ToDto())
	}

	adopted := make([]dto.PublishedStrategyDto, 0, len(adoptedPublications))
	for _, publication := range adoptedPublications {
		adopted = append(adopted, publication.ToDto())
	}

	return dto.AvailableStrategiesDto{Mine: mine, Adopted: adopted}, nil
}

// UpdateStrategy rewrites the strategy this write names and hands it back as it now
// stands. Every rule that governs a new strategy governs a rewritten one, because
// both arrive here as the same shape and are judged by the same model.
//
// A strategy that is out on the marketplace is rewritten like any other. Publishing
// hands out the use of an algorithm, not a frozen copy of it: an owner who fixes a
// mistake should not also have to remember to publish again, and whoever is using
// it should get the fix.
func (strategyService *StrategyService) UpdateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	// No strategy carries no identifier, so there is nothing here to rewrite. Saying
	// so here rather than letting the write go out is the difference between a
	// guarantee this code makes and one it borrows: a rewrite with no identifier
	// names no row, and what an ORM does with a write that names no row is its own
	// decision to change.
	if writeDto.ID == 0 {
		return dto.StrategyDto{}, domains.StrategyNotFound(writeDto.ID)
	}

	// Whether the strategy is there, and whether it is this caller's, are both
	// settled before its content is judged. The other way round, rewriting somebody
	// else's strategy with content that is also wrong answers "a strategy must carry
	// a name" — which tells a stranger their target exists and what is wrong with
	// what they sent.
	if ownershipError := strategyService.requireOwnership(
		executionContext, writeDto.OwnerID, writeDto.ID); ownershipError != nil {
		return dto.StrategyDto{}, ownershipError
	}

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	updatedStrategy, updateError := strategyService.strategyRepository.Update(
		executionContext, strategyDomain.ToEntity())
	if updateError != nil {
		return dto.StrategyDto{}, updateError
	}

	return updatedStrategy.ToDto(), nil
}

// DeleteStrategy removes the viewer's own strategy for good, taking its place on
// the marketplace and everybody's adoption of it along with it.
func (strategyService *StrategyService) DeleteStrategy(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if ownershipError := strategyService.requireOwnership(
		executionContext, viewerID, id); ownershipError != nil {
		return ownershipError
	}

	return strategyService.strategyRepository.Delete(executionContext, id)
}

// ResolveRunnableStrategy turns an identifier into the algorithm, the knobs and the
// kind of value behind it, for whoever is allowed to run it.
//
// This is where the three gates are walked in full, and the only way a script
// leaves storage for a run. What comes back goes to the application layer and stops
// there, so running somebody else's strategy never becomes a way to read it.
//
// Whether the caller has adopted it is not asked. Adoption fills a picker; it does
// not grant anything, or choosing from the marketplace would mean choosing blind.
func (strategyService *StrategyService) ResolveRunnableStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyDto, error) {
	strategy, findError := strategyService.strategyRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.RunnableStrategyDto{}, findError
	}

	isPublished, publicationError := strategyService.isPublished(executionContext, id)
	if publicationError != nil {
		return dto.RunnableStrategyDto{}, publicationError
	}

	return domains.NewStrategyAccessDomain(strategy, viewerID, isPublished).ToRunnableDto()
}

// requireOwnership is the two steps that stand in front of changing a strategy:
// find it, then ask whether it is this caller's. Rewriting and deleting both need
// them, and both owe a stranger the same sentence as a strategy that is not there.
//
// It stops at the second gate on purpose. Publishing hands out the use of an
// algorithm, never the right to alter it, so whether the strategy is on the
// marketplace cannot change this answer — and not asking saves a read.
func (strategyService *StrategyService) requireOwnership(
	executionContext context.Context, viewerID uint, id uint,
) error {
	strategy, findError := strategyService.strategyRepository.FindOne(executionContext, id)
	if findError != nil {
		return findError
	}

	return domains.NewStrategyAccessDomain(strategy, viewerID, false).RequireOwnership()
}

// isPublished answers the third gate. "There is no publication" is not a failure to
// report upwards — it is one of the two answers — so it is read here and turned
// into a plain no.
func (strategyService *StrategyService) isPublished(
	executionContext context.Context, strategyID uint,
) (bool, error) {
	_, findError := strategyService.publishedStrategyRepository.FindOne(executionContext, strategyID)
	if errors.Is(findError, domains.ErrStrategyNotPublished) {
		return false, nil
	}
	if findError != nil {
		return false, findError
	}

	return true, nil
}
