package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyScriptApplication joins the strategy script and bot services, which do not know about each other.
type StrategyScriptApplication struct {
	strategyScriptService *service.StrategyScriptService
	strategyBotService    *service.StrategyBotService
}

func NewStrategyScriptApplication(
	strategyScriptService *service.StrategyScriptService, strategyBotService *service.StrategyBotService,
) *StrategyScriptApplication {
	return &StrategyScriptApplication{
		strategyScriptService: strategyScriptService,
		strategyBotService:    strategyBotService,
	}
}

func (strategyScriptApplication *StrategyScriptApplication) CreateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.CreateStrategyScript(executionContext, writeDto)
}

func (strategyScriptApplication *StrategyScriptApplication) GetStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.GetStrategyScript(executionContext, viewerID, id)
}

func (strategyScriptApplication *StrategyScriptApplication) ListAvailableStrategyScripts(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategyScriptsDto, error) {
	return strategyScriptApplication.strategyScriptService.ListAvailableStrategyScripts(executionContext, viewerID)
}

// UpdateStrategyScript refuses while any of the owner's running bots uses the script, naming them, for the
// same reason a followed trading strategy is refused; stopped bots pick up the new version on next start.
func (strategyScriptApplication *StrategyScriptApplication) UpdateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	// An ID-less rewrite goes straight to the service, which refuses it before any read.
	if writeDto.ID == 0 {
		return strategyScriptApplication.strategyScriptService.UpdateStrategyScript(executionContext, writeDto)
	}

	// Checked first so a stranger's script is refused with the usual not-found sentence, and a running bot
	// is reported before validation errors.
	if _, findError := strategyScriptApplication.strategyScriptService.GetStrategyScript(
		executionContext, writeDto.OwnerID, writeDto.ID); findError != nil {
		return dto.StrategyScriptDto{}, findError
	}

	references, referencesError := strategyScriptApplication.strategyBotService.ReadReferencesToStrategyScript(
		executionContext, writeDto.OwnerID, writeDto.ID)
	if referencesError != nil {
		return dto.StrategyScriptDto{}, referencesError
	}

	if len(references.RunningBotNames) > 0 {
		return dto.StrategyScriptDto{}, domains.StrategyScriptBotRunning(references.RunningBotNames)
	}

	return strategyScriptApplication.strategyScriptService.UpdateStrategyScript(executionContext, writeDto)
}

func (strategyScriptApplication *StrategyScriptApplication) DeleteStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return strategyScriptApplication.strategyScriptService.DeleteStrategyScript(executionContext, viewerID, id)
}

// ResolveRunnableStrategyScript returns the algorithm behind an identifier for whoever may run it; it is exported for cross-service orchestration and its result never reaches a controller.
func (strategyScriptApplication *StrategyScriptApplication) ResolveRunnableStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.ResolveRunnableStrategyScript(executionContext, viewerID, id)
}
