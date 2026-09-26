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
	clockProxy                        domaininterface.IClockProxy
}

func NewStrategyScriptMarketplaceService(
	strategyScriptRepository domaininterface.IStrategyScriptRepository,
	publishedStrategyScriptRepository domaininterface.IPublishedStrategyScriptRepository,
	clockProxy domaininterface.IClockProxy,
) *StrategyScriptMarketplaceService {
	return &StrategyScriptMarketplaceService{
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
		clockProxy:                        clockProxy,
	}
}

// PublishStrategyScript puts the owner's script on the marketplace; republishing keeps its original publish time, and non-owners are told it does not exist.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) PublishStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	strategyScript, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindOne(
		executionContext, strategyScriptID)
	if findError != nil {
		return findError
	}

	if rewritableError := domains.NewStrategyScriptAccessDomain(strategyScript, ownerID, false).
		RequireRewritable(domains.StrategyScriptFromMarketplaceNotRepublishable()); rewritableError != nil {
		return rewritableError
	}

	return strategyScriptMarketplaceService.publishedStrategyScriptRepository.Publish(
		executionContext, strategyScriptID, strategyScriptMarketplaceService.clockProxy.Now())
}

// WithdrawStrategyScript takes the owner's script off the marketplace; copies already adopted are untouched.
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

// AdoptStrategyScript gives the adopter a snapshot copy of their own, so the author can no longer change what the
// adopter's bots run; adopting one's own script does nothing, and a name the adopter already holds is refused.
func (strategyScriptMarketplaceService *StrategyScriptMarketplaceService) AdoptStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	original, findError := strategyScriptMarketplaceService.strategyScriptRepository.FindOne(
		executionContext, strategyScriptID)
	if findError != nil {
		return findError
	}

	access := domains.NewStrategyScriptAccessDomain(original, userID, original.Publication != nil)
	if access.IsOwnedByViewer() {
		return nil
	}
	if !access.IsRunnable() {
		return domains.StrategyScriptNotFound(strategyScriptID)
	}

	_, saveError := strategyScriptMarketplaceService.strategyScriptRepository.Save(
		executionContext,
		domains.NewStrategyScriptMarketplaceCopyDomain(
			original, userID, strategyScriptMarketplaceService.clockProxy.Now()).ToEntity())

	return saveError
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
