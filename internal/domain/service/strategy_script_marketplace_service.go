package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

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

// PublishStrategyScript puts the owner's script on the marketplace; republishing keeps its original publish time, and non-owners are told it does not exist.
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

// WithdrawStrategyScript takes the owner's script off the marketplace and clears every adoption, so republishing starts a fresh decision for each person.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) WithdrawStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	if ownershipError := strategyScriptMarketplaceService.requireOwnership(
		executionContext, ownerID, strategyScriptID); ownershipError != nil {
		return ownershipError
	}

	return strategyScriptMarketplaceService.publishedStrategyScriptRepository.Withdraw(executionContext, strategyScriptID)
}

// BrowseMarketplace returns every published script without its source, newest first, including the viewer's own.
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

// AdoptStrategyScript adds a published script to the user's shelf idempotently; adopting one's own script is a no-op.
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

// AbandonStrategyScript removes a script from the user's shelf only, without looking the script up, so tidying up still works after it was deleted.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) AbandonStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceService.strategyScriptAdoptionRepository.Abandon(executionContext, userID, strategyScriptID)
}

// requireOwnership answers a stranger exactly as for a missing script.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) requireOwnership(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	strategyScript, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindOne(executionContext, strategyScriptID)
	if findError != nil {
		return findError
	}

	return domains.NewStrategyScriptAccessDomain(strategyScript, ownerID, false).RequireOwnership()
}
