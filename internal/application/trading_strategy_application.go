package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingStrategyApplication orchestrates everything a person does to a trading
// strategy.
//
// It joins two domain services, which is this layer's job and not theirs. A trading
// strategy names strategy scripts, so saving one has to ask the strategy script
// rules whether those may be seen and what knobs they declare; changing or deleting
// one has to ask the bots whether anything is following it. Neither question belongs
// to the trading strategies, and neither service knows the other exists.
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

// CreateTradingStrategy saves a new set of rules for whoever is signed in.
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

// ListTradingStrategies returns this person's trading strategies.
func (tradingStrategyApplication *TradingStrategyApplication) ListTradingStrategies(
	executionContext context.Context, viewerID uint,
) ([]dto.TradingStrategyDto, error) {
	return tradingStrategyApplication.tradingStrategyService.ListTradingStrategies(
		executionContext, viewerID)
}

// GetTradingStrategy returns one of this person's trading strategies.
func (tradingStrategyApplication *TradingStrategyApplication) GetTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.TradingStrategyDto, error) {
	return tradingStrategyApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, id)
}

// UpdateTradingStrategy rewrites one of this person's trading strategies, and
// refuses while any bot following it is running.
//
// The refusal names those bots. A round that began under one version of the rules
// and ended under another leaves nobody able to say which version it used, and the
// only way out is to stop those bots — which somebody can only do if they are told
// which ones they are.
//
// Bots that are merely stopped are no reason to refuse. They pick the new rules up
// the next time they are started, and that is the whole point of several bots
// sharing one set.
func (tradingStrategyApplication *TradingStrategyApplication) UpdateTradingStrategy(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	// Asked before anything else, so that somebody whose bot is running is told to
	// stop it rather than told about a typo they would then fix for nothing.
	//
	// Reading it first also settles that this trading strategy is theirs: a
	// stranger's is refused here with the same sentence every other path uses.
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

// DeleteTradingStrategy removes one of this person's trading strategies, and refuses
// while any bot still follows it — running or not.
//
// Running is not the question here, as it is for a rewrite. Deleting would leave
// those bots pointing at something that is gone, and a bot that cannot reach its
// rules is indistinguishable from a broken one whether it is switched on or not.
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

// withResolvedStrategyScripts fills in, for every source, the one thing only the
// strategy script itself can say: the knobs it declares.
//
// Resolving is also the gate. Naming a strategy script that is not this person's and
// not on the marketplace fails here with the same sentence as naming one that does
// not exist, which is what stops a trading strategy's sources becoming a way to
// probe for strategy scripts.
//
// Both creating and rewriting need every step of this, which is what earns it a name
// of its own.
func (tradingStrategyApplication *TradingStrategyApplication) withResolvedStrategyScripts(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyWriteDto, error) {
	resolvedSources := make(
		[]dto.TradingStrategySignalSourceWriteDto, 0, len(writeDto.SignalSources))

	for _, signalSource := range writeDto.SignalSources {
		runnableStrategyScript, resolveError := tradingStrategyApplication.strategyScriptService.
			ResolveRunnableStrategyScript(executionContext, viewerID, signalSource.StrategyScriptID)
		if resolveError != nil {
			return dto.TradingStrategyWriteDto{}, resolveError
		}

		// Only the declared knobs and the kind of value it produces are taken. The
		// script is deliberately left behind: what a trading strategy stores about a
		// source is which strategy script it names, so that a script adopted from the
		// marketplace is run without ever being copied somewhere its adopter could
		// read it.
		signalSource.DeclaredParameters = runnableStrategyScript.Parameters
		signalSource.DeclaredResultType = runnableStrategyScript.ResultType
		signalSource.DeclaredMarketDataKind = runnableStrategyScript.MarketDataKind
		resolvedSources = append(resolvedSources, signalSource)
	}

	writeDto.SignalSources = resolvedSources

	return writeDto, nil
}
