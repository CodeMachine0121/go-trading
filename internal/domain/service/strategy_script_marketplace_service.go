package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyScriptMarketplaceService is the application layer's only entry point for the
// shared shelf: putting a strategy script on it, taking one off, looking at it, and
// keeping one's own selection from it. Its public use-case methods never call one
// another.
//
// It is a separate service from StrategyScriptService rather than five more methods on
// it, because the two answer different questions. StrategyScriptService answers "what is
// mine and what may I run"; this one answers "what is out there and what have I
// taken". They happen to read the same table, which is not the same as being the
// same concern — and the day the marketplace grows ratings or categories, only one
// of the two grows with it.
type StrategyScriptMarketplaceService struct {
	strategyScriptRepository          domaininterface.IStrategyScriptRepository
	publishedStrategyScriptRepository domaininterface.IPublishedStrategyScriptRepository
	strategyScriptAdoptionRepository  domaininterface.IStrategyScriptAdoptionRepository
	clockProxy                        domaininterface.IClockProxy
}

func NewStrategyScriptMarketplaceService(
	strategyScriptRepository domaininterface.IStrategyScriptRepository,
	publishedStrategyScriptRepository domaininterface.IPublishedStrategyScriptRepository,
	strategyScriptAdoptionRepository domaininterface.IStrategyScriptAdoptionRepository,
	clockProxy domaininterface.IClockProxy,
) *StrategyScriptMarketplaceService {
	return &StrategyScriptMarketplaceService{
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
		strategyScriptAdoptionRepository:  strategyScriptAdoptionRepository,
		clockProxy:                        clockProxy,
	}
}

// PublishStrategyScript puts the owner's strategy script on the marketplace. Publishing one that
// is already there is not a failure and does not move the moment it first arrived:
// it has been out since then, and saying so twice does not change when it started.
//
// Only the owner may publish, and anyone else is told the strategy script is not there —
// the same sentence they would get for one that never existed.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) PublishStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	if ownershipError := strategyScriptMarketplaceService.requireOwnership(
		executionContext, ownerID, strategyScriptID); ownershipError != nil {
		return ownershipError
	}

	return strategyScriptMarketplaceService.publishedStrategyScriptRepository.Publish(
		executionContext, strategyScriptID, strategyScriptMarketplaceService.clockProxy.Now())
}

// WithdrawStrategyScript takes the owner's strategy script off the marketplace, clearing every
// adoption of it as it goes.
//
// Withdrawing one that was never there is not a failure — what was asked for is
// already true. Clearing the adoptions rather than leaving them to fail quietly is
// deliberate: an owner who takes something back has not agreed that everyone gets
// it again the moment they change their mind, so publishing it again starts a fresh
// decision for each person.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) WithdrawStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	if ownershipError := strategyScriptMarketplaceService.requireOwnership(
		executionContext, ownerID, strategyScriptID); ownershipError != nil {
		return ownershipError
	}

	return strategyScriptMarketplaceService.publishedStrategyScriptRepository.Withdraw(executionContext, strategyScriptID)
}

// BrowseMarketplace returns everything on the shared shelf, newest first, each one
// without its script. An empty shelf is an answer, not a failure.
//
// It asks who is looking for nothing, because a published strategy script is public. A
// person's own published strategy scripts appear here too — the marketplace shows what is
// out there, and theirs is out there.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) BrowseMarketplace(
	executionContext context.Context,
) ([]dto.PublishedStrategyScriptDto, error) {
	publications, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindAllPublished(executionContext)
	if findError != nil {
		return nil, findError
	}

	publishedStrategyScriptDtos := make([]dto.PublishedStrategyScriptDto, 0, len(publications))
	for _, publication := range publications {
		publishedStrategyScriptDtos = append(publishedStrategyScriptDtos, publication.ToDto())
	}

	return publishedStrategyScriptDtos, nil
}

// AdoptStrategyScript puts a published strategy script on this person's own shelf. Taking one
// twice leaves one entry.
//
// Adopting one's own strategy script does nothing and is not a failure: it is already on
// the shelf, and the request asks for a state that already holds. Something that is
// not on the marketplace is refused with the one sentence a closed door gives.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) AdoptStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	strategyScript, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindOne(executionContext, strategyScriptID)
	if findError != nil {
		return findError
	}

	if domains.NewStrategyScriptAccessDomain(strategyScript, userID, false).IsOwnedByViewer() {
		return nil
	}

	return strategyScriptMarketplaceService.strategyScriptAdoptionRepository.Adopt(
		executionContext, userID, strategyScriptID, strategyScriptMarketplaceService.clockProxy.Now())
}

// AbandonStrategyScript takes a strategy script off this person's own shelf. It touches nobody
// else: the strategy script stays on the marketplace and everyone else keeps theirs.
// Dropping one that was never taken is not a failure.
//
// It asks nothing about the strategy script first. A shelf entry belongs to the person
// whose shelf it is, so removing one needs no permission from anybody — and a
// strategy script that has since been deleted outright would make a lookup fail on a
// request that is only trying to tidy up.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) AbandonStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceService.strategyScriptAdoptionRepository.Abandon(executionContext, userID, strategyScriptID)
}

// requireOwnership is the second gate, shared by publishing and withdrawing. Both
// are things only an owner does to their own strategy script, and both owe a stranger the
// same sentence as a strategy script that is not there.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) requireOwnership(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	strategyScript, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindOne(executionContext, strategyScriptID)
	if findError != nil {
		return findError
	}

	return domains.NewStrategyScriptAccessDomain(strategyScript, ownerID, false).RequireOwnership()
}
