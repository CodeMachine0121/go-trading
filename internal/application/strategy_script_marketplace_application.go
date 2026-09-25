package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type StrategyScriptMarketplaceApplication struct {
	strategyScriptMarketplaceService *service.StrategyScriptMarketplaceService
}

func NewStrategyScriptMarketplaceApplication(
	strategyScriptMarketplaceService *service.StrategyScriptMarketplaceService,
) *StrategyScriptMarketplaceApplication {
	return &StrategyScriptMarketplaceApplication{strategyScriptMarketplaceService: strategyScriptMarketplaceService}
}

func (strategyScriptMarketplaceApplication *StrategyScriptMarketplaceApplication) PublishStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceApplication.strategyScriptMarketplaceService.PublishStrategyScript(
		executionContext, ownerID, strategyScriptID)
}

func (strategyScriptMarketplaceApplication *StrategyScriptMarketplaceApplication) WithdrawStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceApplication.strategyScriptMarketplaceService.WithdrawStrategyScript(
		executionContext, ownerID, strategyScriptID)
}

func (strategyScriptMarketplaceApplication *StrategyScriptMarketplaceApplication) BrowseMarketplace(
	executionContext context.Context,
) ([]dto.PublishedStrategyScriptDto, error) {
	return strategyScriptMarketplaceApplication.strategyScriptMarketplaceService.BrowseMarketplace(executionContext)
}

func (strategyScriptMarketplaceApplication *StrategyScriptMarketplaceApplication) AdoptStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceApplication.strategyScriptMarketplaceService.AdoptStrategyScript(
		executionContext, userID, strategyScriptID)
}

func (strategyScriptMarketplaceApplication *StrategyScriptMarketplaceApplication) AbandonStrategyScript(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	return strategyScriptMarketplaceApplication.strategyScriptMarketplaceService.AbandonStrategyScript(
		executionContext, userID, strategyScriptID)
}
