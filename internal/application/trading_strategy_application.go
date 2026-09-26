package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingStrategyApplication joins the trading strategy, strategy script and bot services, which do
// not know about each other.
type TradingStrategyApplication struct {
	tradingStrategyService *service.TradingStrategyService
	strategyScriptService  *service.StrategyScriptService
	strategyBotService     *service.StrategyBotService
}

func NewTradingStrategyApplication(
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	strategyBotService *service.StrategyBotService,
) *TradingStrategyApplication {
	return &TradingStrategyApplication{
		tradingStrategyService: tradingStrategyService,
		strategyScriptService:  strategyScriptService,
		strategyBotService:     strategyBotService,
	}
}

func (tradingStrategyApplication *TradingStrategyApplication) CreateTradingStrategy(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	writeDto.OwnerID = viewerID

	resolvedWriteDto, resolveError := tradingStrategyApplication.withResolvedStrategyScripts(
		executionContext, viewerID, writeDto)
	if resolveError != nil {
		return dto.TradingStrategyDto{}, resolveError
	}

	return tradingStrategyApplication.tradingStrategyService.CreateTradingStrategy(
		executionContext, resolvedWriteDto)
}

func (tradingStrategyApplication *TradingStrategyApplication) ListTradingStrategies(
	executionContext context.Context, viewerID uint,
) ([]dto.TradingStrategyDto, error) {
	return tradingStrategyApplication.tradingStrategyService.ListTradingStrategies(
		executionContext, viewerID)
}

func (tradingStrategyApplication *TradingStrategyApplication) GetTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.TradingStrategyDto, error) {
	return tradingStrategyApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, id)
}

// UpdateTradingStrategy refuses while any bot following the strategy is running, naming those bots,
// since a round must not straddle two versions of the rules; stopped bots simply pick up the new
// rules on next start.
func (tradingStrategyApplication *TradingStrategyApplication) UpdateTradingStrategy(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	// Checked first so a running bot is reported before validation errors, and so a stranger's
	// strategy is refused with the usual not-found sentence.
	if _, findError := tradingStrategyApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, writeDto.ID); findError != nil {
		return dto.TradingStrategyDto{}, findError
	}

	references, referencesError := tradingStrategyApplication.strategyBotService.ReadReferencesTo(
		executionContext, writeDto.ID)
	if referencesError != nil {
		return dto.TradingStrategyDto{}, referencesError
	}

	if len(references.RunningBotNames) > 0 {
		return dto.TradingStrategyDto{}, domains.TradingStrategyBotRunning(references.RunningBotNames)
	}

	resolvedWriteDto, resolveError := tradingStrategyApplication.withResolvedStrategyScripts(
		executionContext, viewerID, writeDto)
	if resolveError != nil {
		return dto.TradingStrategyDto{}, resolveError
	}

	return tradingStrategyApplication.tradingStrategyService.UpdateTradingStrategy(
		executionContext, viewerID, resolvedWriteDto)
}

// InspectTradingStrategyRewrite applies the rewrite's rules without writing and says how many bots follow the
// strategy; running bots are not refused here, since the owner can stop them before the rewrite is carried out.
func (tradingStrategyApplication *TradingStrategyApplication) InspectTradingStrategyRewrite(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.RewriteTargetDto, error) {
	// Checked first so a stranger's strategy is refused with the usual not-found sentence.
	if _, findError := tradingStrategyApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, writeDto.ID); findError != nil {
		return dto.RewriteTargetDto{}, findError
	}

	resolvedWriteDto, resolveError := tradingStrategyApplication.withResolvedStrategyScripts(
		executionContext, viewerID, writeDto)
	if resolveError != nil {
		return dto.RewriteTargetDto{}, resolveError
	}

	tradingStrategyDto, inspectError := tradingStrategyApplication.tradingStrategyService.
		InspectTradingStrategyRewrite(executionContext, viewerID, resolvedWriteDto)
	if inspectError != nil {
		return dto.RewriteTargetDto{}, inspectError
	}

	references, referencesError := tradingStrategyApplication.strategyBotService.ReadReferencesTo(
		executionContext, writeDto.ID)
	if referencesError != nil {
		return dto.RewriteTargetDto{}, referencesError
	}

	return dto.RewriteTargetDto{
		ID:                tradingStrategyDto.ID,
		Name:              tradingStrategyDto.Name,
		UpdatedAt:         tradingStrategyDto.UpdatedAt,
		BotReferenceCount: references.TotalCount,
	}, nil
}

// DeleteTradingStrategy refuses while any bot, running or not, still follows the strategy, since it
// would be left pointing at nothing.
func (tradingStrategyApplication *TradingStrategyApplication) DeleteTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := tradingStrategyApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, id); findError != nil {
		return findError
	}

	references, referencesError := tradingStrategyApplication.strategyBotService.ReadReferencesTo(
		executionContext, id)
	if referencesError != nil {
		return referencesError
	}

	if references.TotalCount > 0 {
		return domains.TradingStrategyInUse(references.TotalCount)
	}

	return tradingStrategyApplication.tradingStrategyService.DeleteTradingStrategy(
		executionContext, viewerID, id)
}

// withResolvedStrategyScripts copies each script's declared knobs into its source and doubles as
// the access gate: an unreadable script fails like a missing one, so sources cannot probe for
// scripts.
func (tradingStrategyApplication *TradingStrategyApplication) withResolvedStrategyScripts(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyWriteDto, error) {
	resolvedSources := make(
		[]dto.TradingStrategySignalSourceWriteDto, 0, len(writeDto.SignalSources))

	for _, signalSource := range writeDto.SignalSources {
		runnableStrategyScript, resolveError := tradingStrategyApplication.strategyScriptService.
			ResolveOwnedStrategyScript(executionContext, viewerID, signalSource.StrategyScriptID)
		if resolveError != nil {
			return dto.TradingStrategyWriteDto{}, resolveError
		}

		// Only the declarations are copied, never the script body, so an adopted marketplace script
		// stays unreadable to its adopter.
		signalSource.DeclaredParameters = runnableStrategyScript.Parameters
		signalSource.DeclaredResultType = runnableStrategyScript.ResultType
		signalSource.DeclaredMarketDataKind = runnableStrategyScript.MarketDataKind
		resolvedSources = append(resolvedSources, signalSource)
	}

	writeDto.SignalSources = resolvedSources

	return writeDto, nil
}
