package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyMarketplaceService is the application layer's only entry point for the
// shared shelf: putting a strategy on it, taking one off, looking at it, and
// keeping one's own selection from it. Its public use-case methods never call one
// another.
//
// It is a separate service from StrategyService rather than five more methods on
// it, because the two answer different questions. StrategyService answers "what is
// mine and what may I run"; this one answers "what is out there and what have I
// taken". They happen to read the same table, which is not the same as being the
// same concern — and the day the marketplace grows ratings or categories, only one
// of the two grows with it.
type StrategyMarketplaceService struct {
	strategyRepository          domaininterface.IStrategyRepository
	publishedStrategyRepository domaininterface.IPublishedStrategyRepository
	strategyAdoptionRepository  domaininterface.IStrategyAdoptionRepository
	clockProxy                  domaininterface.IClockProxy
}

func NewStrategyMarketplaceService(
	strategyRepository domaininterface.IStrategyRepository,
	publishedStrategyRepository domaininterface.IPublishedStrategyRepository,
	strategyAdoptionRepository domaininterface.IStrategyAdoptionRepository,
	clockProxy domaininterface.IClockProxy,
) *StrategyMarketplaceService {
	return &StrategyMarketplaceService{
		strategyRepository:          strategyRepository,
		publishedStrategyRepository: publishedStrategyRepository,
		strategyAdoptionRepository:  strategyAdoptionRepository,
		clockProxy:                  clockProxy,
	}
}

// PublishStrategy puts the owner's strategy on the marketplace. Publishing one that
// is already there is not a failure and does not move the moment it first arrived:
// it has been out since then, and saying so twice does not change when it started.
//
// Only the owner may publish, and anyone else is told the strategy is not there —
// the same sentence they would get for one that never existed.
func (strategyMarketplaceService *StrategyMarketplaceService) PublishStrategy(
	executionContext context.Context, ownerID uint, strategyID uint,
) error {
	if ownershipError := strategyMarketplaceService.requireOwnership(
		executionContext, ownerID, strategyID); ownershipError != nil {
		return ownershipError
	}

	return strategyMarketplaceService.publishedStrategyRepository.Publish(
		executionContext, strategyID, strategyMarketplaceService.clockProxy.Now())
}

// WithdrawStrategy takes the owner's strategy off the marketplace, clearing every
// adoption of it as it goes.
//
// Withdrawing one that was never there is not a failure — what was asked for is
// already true. Clearing the adoptions rather than leaving them to fail quietly is
// deliberate: an owner who takes something back has not agreed that everyone gets
// it again the moment they change their mind, so publishing it again starts a fresh
// decision for each person.
func (strategyMarketplaceService *StrategyMarketplaceService) WithdrawStrategy(
	executionContext context.Context, ownerID uint, strategyID uint,
) error {
	if ownershipError := strategyMarketplaceService.requireOwnership(
		executionContext, ownerID, strategyID); ownershipError != nil {
		return ownershipError
	}

	return strategyMarketplaceService.publishedStrategyRepository.Withdraw(executionContext, strategyID)
}

// BrowseMarketplace returns everything on the shared shelf, newest first, each one
// without its script. An empty shelf is an answer, not a failure.
//
// It asks who is looking for nothing, because a published strategy is public. A
// person's own published strategies appear here too — the marketplace shows what is
// out there, and theirs is out there.
func (strategyMarketplaceService *StrategyMarketplaceService) BrowseMarketplace(
	executionContext context.Context,
) ([]dto.PublishedStrategyDto, error) {
	publications, findError := strategyMarketplaceService.strategyRepository.FindAllPublished(executionContext)
	if findError != nil {
		return nil, findError
	}

	return publishedStrategyDtosOf(publications), nil
}

// AdoptStrategy puts a published strategy on this person's own shelf. Taking one
// twice leaves one entry.
//
// Adopting one's own strategy does nothing and is not a failure: it is already on
// the shelf, and the request asks for a state that already holds. Something that is
// not on the marketplace is refused with the one sentence a closed door gives.
func (strategyMarketplaceService *StrategyMarketplaceService) AdoptStrategy(
	executionContext context.Context, userID uint, strategyID uint,
) error {
	strategy, findError := strategyMarketplaceService.strategyRepository.FindOne(executionContext, strategyID)
	if findError != nil {
		return findError
	}

	if domains.NewStrategyAccessDomain(strategy, userID, false).IsOwnedByViewer() {
		return nil
	}

	return strategyMarketplaceService.strategyAdoptionRepository.Adopt(
		executionContext, userID, strategyID, strategyMarketplaceService.clockProxy.Now())
}

// AbandonStrategy takes a strategy off this person's own shelf. It touches nobody
// else: the strategy stays on the marketplace and everyone else keeps theirs.
// Dropping one that was never taken is not a failure.
//
// It asks nothing about the strategy first. A shelf entry belongs to the person
// whose shelf it is, so removing one needs no permission from anybody — and a
// strategy that has since been deleted outright would make a lookup fail on a
// request that is only trying to tidy up.
func (strategyMarketplaceService *StrategyMarketplaceService) AbandonStrategy(
	executionContext context.Context, userID uint, strategyID uint,
) error {
	return strategyMarketplaceService.strategyAdoptionRepository.Abandon(executionContext, userID, strategyID)
}

// requireOwnership is the second gate, shared by publishing and withdrawing. Both
// are things only an owner does to their own strategy, and both owe a stranger the
// same sentence as a strategy that is not there.
func (strategyMarketplaceService *StrategyMarketplaceService) requireOwnership(
	executionContext context.Context, ownerID uint, strategyID uint,
) error {
	strategy, findError := strategyMarketplaceService.strategyRepository.FindOne(executionContext, strategyID)
	if findError != nil {
		return findError
	}

	return domains.NewStrategyAccessDomain(strategy, ownerID, false).RequireOwnership()
}
