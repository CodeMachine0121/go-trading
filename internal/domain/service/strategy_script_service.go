package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// It deliberately cannot reach K candles, and consults the marketplace only for the third access gate: is the script published.
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

// CreateStrategyScript saves a new script, refusing one that breaks a rule (including having no owner) before anything is written.
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

// GetStrategyScript returns the viewer's own script with its source; others' published scripts are read from the marketplace, which returns a shape without the source.
func (strategyScriptService *StrategyScriptService) GetStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyScriptDto, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyScriptDto{}, findError
	}

	return domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false).ToOwnerDto()
}

// ListAvailableStrategyScripts splits the viewer's scripts into their own work and their marketplace copies.
func (strategyScriptService *StrategyScriptService) ListAvailableStrategyScripts(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategyScriptsDto, error) {
	ownStrategyScripts, ownError := strategyScriptService.strategyScriptRepository.FindAllOwnedBy(executionContext, viewerID)
	if ownError != nil {
		return dto.AvailableStrategyScriptsDto{}, ownError
	}

	mine := make([]dto.StrategyScriptDto, 0, len(ownStrategyScripts))
	adopted := make([]dto.StrategyScriptDto, 0)
	for _, strategyScript := range ownStrategyScripts {
		if domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false).IsAdoptedFromMarketplace() {
			adopted = append(adopted, strategyScript.ToDto())
			continue
		}

		mine = append(mine, strategyScript.ToDto())
	}

	return dto.AvailableStrategyScriptsDto{Mine: mine, Adopted: adopted}, nil
}

// UpdateStrategyScript rewrites a script under the create rules; a published script is rewritten too, since publishing shares the algorithm rather than a frozen copy.
func (strategyScriptService *StrategyScriptService) UpdateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	_, strategyScriptDomain, rewriteError := strategyScriptService.preparedRewrite(executionContext, writeDto)
	if rewriteError != nil {
		return dto.StrategyScriptDto{}, rewriteError
	}

	updatedStrategyScript, updateError := strategyScriptService.strategyScriptRepository.Update(
		executionContext, strategyScriptDomain.ToEntity())
	if updateError != nil {
		return dto.StrategyScriptDto{}, updateError
	}

	return updatedStrategyScript.ToDto(), nil
}

// InspectStrategyScriptRewrite applies every rule a rewrite would, writes nothing, and returns the script as it stands.
func (strategyScriptService *StrategyScriptService) InspectStrategyScriptRewrite(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	existingStrategyScript, _, rewriteError := strategyScriptService.preparedRewrite(executionContext, writeDto)
	if rewriteError != nil {
		return dto.StrategyScriptDto{}, rewriteError
	}

	return existingStrategyScript.ToDto(), nil
}

// preparedRewrite checks a rewrite in the order that reveals nothing to a stranger: identifier, ownership, then content.
func (strategyScriptService *StrategyScriptService) preparedRewrite(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (entities.StrategyScript, domains.StrategyScriptDomain, error) {
	// Refuse an ID-less rewrite here instead of relying on what the ORM does with a write that names no row.
	if writeDto.ID == 0 {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, domains.StrategyScriptNotFound(writeDto.ID)
	}

	existingStrategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(
		executionContext, writeDto.ID)
	if findError != nil {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, findError
	}

	if rewritableError := domains.NewStrategyScriptAccessDomain(existingStrategyScript, writeDto.OwnerID, false).
		RequireRewritable(domains.StrategyScriptFromMarketplaceNotRewritable()); rewritableError != nil {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, rewritableError
	}

	// The market data kind is judged against the stored one: omitting it keeps it, changing it is refused.
	existingMarketDataKind, existingKindError := domains.NewMarketDataKindDomain(
		existingStrategyScript.MarketDataKind)
	if existingKindError != nil {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, existingKindError
	}

	marketDataKind, retainingError := existingMarketDataKind.Retaining(writeDto.MarketDataKind)
	if retainingError != nil {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, retainingError
	}
	writeDto.MarketDataKind = string(marketDataKind.Value())

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)
	if validationError != nil {
		return entities.StrategyScript{}, domains.StrategyScriptDomain{}, validationError
	}

	return existingStrategyScript, strategyScriptDomain, nil
}

// DeleteStrategyScript removes the viewer's script along with its marketplace listing; copies others adopted stay.
func (strategyScriptService *StrategyScriptService) DeleteStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, ownershipError := strategyScriptService.requireOwnership(
		executionContext, viewerID, id); ownershipError != nil {
		return ownershipError
	}

	return strategyScriptService.strategyScriptRepository.Delete(executionContext, id)
}

// ResolveRunnableStrategyScript lets a one-off run use the viewer's own script or someone else's published one, so a marketplace script can be tried before it is adopted.
func (strategyScriptService *StrategyScriptService) ResolveRunnableStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.RunnableStrategyScriptDto{}, findError
	}

	// Owners skip the publication read: bots resolve their own scripts every round, so it would be a wasted query (and a spurious "record not found" log) per source per round.
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

// ResolveOwnedStrategyScript is the gate for anything a bot can depend on: only the viewer's own scripts, copies
// included, so no author can change another person's rules.
func (strategyScriptService *StrategyScriptService) ResolveOwnedStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.RunnableStrategyScriptDto{}, findError
	}

	return domains.NewStrategyScriptAccessDomain(
		strategyScript, viewerID, strategyScript.Publication != nil).ToOwnedRunnableDto()
}

// requireOwnership answers a stranger exactly as for a missing script and skips the publication gate, since publishing never grants the right to alter.
// It returns the script so a rewrite can check the stored market data kind.
func (strategyScriptService *StrategyScriptService) requireOwnership(
	executionContext context.Context, viewerID uint, id uint,
) (entities.StrategyScript, error) {
	strategyScript, findError := strategyScriptService.strategyScriptRepository.FindOne(executionContext, id)
	if findError != nil {
		return entities.StrategyScript{}, findError
	}

	return strategyScript, domains.NewStrategyScriptAccessDomain(strategyScript, viewerID, false).RequireOwnership()
}

// isPublished turns the not-published sentinel into a plain false.
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
