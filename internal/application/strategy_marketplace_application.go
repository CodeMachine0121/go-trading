package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyMarketplaceApplication orchestrates the shared shelf: publishing,
// withdrawing, browsing, and one person's selection from it.
type StrategyMarketplaceApplication struct {
	strategyMarketplaceService *service.StrategyMarketplaceService
}

func NewStrategyMarketplaceApplication(
	strategyMarketplaceService *service.StrategyMarketplaceService,
) *StrategyMarketplaceApplication {
	return &StrategyMarketplaceApplication{strategyMarketplaceService: strategyMarketplaceService}
}

func (strategyMarketplaceApplication *StrategyMarketplaceApplication) PublishStrategy(
	executionContext context.Context, ownerID uint, strategyID uint,
) error {
	return strategyMarketplaceApplication.strategyMarketplaceService.PublishStrategy(
		executionContext, ownerID, strategyID)
}

func (strategyMarketplaceApplication *StrategyMarketplaceApplication) WithdrawStrategy(
	executionContext context.Context, ownerID uint, strategyID uint,
) error {
	return strategyMarketplaceApplication.strategyMarketplaceService.WithdrawStrategy(
		executionContext, ownerID, strategyID)
}

func (strategyMarketplaceApplication *StrategyMarketplaceApplication) BrowseMarketplace(
	executionContext context.Context,
) ([]dto.PublishedStrategyDto, error) {
	return strategyMarketplaceApplication.strategyMarketplaceService.BrowseMarketplace(executionContext)
}

func (strategyMarketplaceApplication *StrategyMarketplaceApplication) AdoptStrategy(
	executionContext context.Context, userID uint, strategyID uint,
) error {
	return strategyMarketplaceApplication.strategyMarketplaceService.AdoptStrategy(
		executionContext, userID, strategyID)
}

func (strategyMarketplaceApplication *StrategyMarketplaceApplication) AbandonStrategy(
	executionContext context.Context, userID uint, strategyID uint,
) error {
	return strategyMarketplaceApplication.strategyMarketplaceService.AbandonStrategy(
		executionContext, userID, strategyID)
}
