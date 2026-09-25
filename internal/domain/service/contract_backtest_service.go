package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractBacktestService is the application layer's only entry point for contract replays on an isolated account; it only sequences the domain steps.
type ContractBacktestService struct {
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	contractPositionStatisticRepository     domaininterface.IContractPositionStatisticRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository
	contractIndicatorScriptProxy            domaininterface.IContractIndicatorScriptProxy
	clockProxy                              domaininterface.IClockProxy
	maxCandleCount                          int
	// replayTimeAllowance matches the spot replay's.
	replayTimeAllowance time.Duration
}

func NewContractBacktestService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	contractPositionStatisticRepository domaininterface.IContractPositionStatisticRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository,
	contractIndicatorScriptProxy domaininterface.IContractIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
	replayTimeAllowance time.Duration,
) *ContractBacktestService {
	return &ContractBacktestService{
		kCandleContractRepository:               kCandleContractRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		contractPositionStatisticRepository:     contractPositionStatisticRepository,
		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository: contractMaintenanceMarginTierRepository,
		contractIndicatorScriptProxy:            contractIndicatorScriptProxy,
		clockProxy:                              clockProxy,
		maxCandleCount:                          maxCandleCount,
		replayTimeAllowance:                     replayTimeAllowance,
	}
}

// RunContractBacktest replays one contract script once per finished bar, each run seeing all bars up to its own; nothing is stored.
func (contractBacktestService *ContractBacktestService) RunContractBacktest(
	executionContext context.Context, requestDto dto.ContractBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	// The allowance covers reading the market as well as running the scripts.
	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, contractBacktestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

	tradingRules, rulesError := contractBacktestService.readTradingRules(replayContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, rulesError)
	}

	contractBacktestDomain, validationError := domains.NewContractBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	alignment, contractKCandles, settlements, readError := contractBacktestService.readReplayBars(
		replayContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, readError)
	}

	perBarIndicatorValues, executionError := contractBacktestService.contractIndicatorScriptProxy.ExecuteForEachCandle(
		replayContext,
		requestDto.Script,
		contractBacktestDomain.ResultType(),
		contractKCandles,
		contractBacktestDomain.Parameters())
	if executionError != nil {
		return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, executionError)
	}

	return contractBacktestDomain.ReplayOver(
		alignment, signalsOf(perBarIndicatorValues), settlements, nil), nil
}

// RunContractTradingStrategyBacktest replays a contract trading strategy by its trading mode; bars are read once so every source sees the identical stretch.
func (contractBacktestService *ContractBacktestService) RunContractTradingStrategyBacktest(
	executionContext context.Context, requestDto dto.ContractTradingStrategyBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	// The allowance covers reading the market as well as running the scripts.
	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, contractBacktestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

	tradingRules, rulesError := contractBacktestService.readTradingRules(replayContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, rulesError)
	}

	strategyBacktestDomain, validationError := domains.NewContractTradingStrategyBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	contractBacktestDomain := strategyBacktestDomain.ContractBacktest()
	alignment, contractKCandles, settlements, readError := contractBacktestService.readReplayBars(
		replayContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, readError)
	}

	signalsBySource := make([][]domains.SignalDomain, 0, strategyBacktestDomain.SourceCount())
	for sourceIndex := range strategyBacktestDomain.SourceCount() {
		perBarIndicatorValues, executionError := contractBacktestService.contractIndicatorScriptProxy.ExecuteForEachCandle(
			replayContext,
			strategyBacktestDomain.SourceScript(sourceIndex),
			contractBacktestDomain.ResultType(),
			contractKCandles,
			strategyBacktestDomain.SourceParameters(sourceIndex))
		// One failing source ends the replay; missing signals would silently make conditions false.
		if executionError != nil {
			return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, executionError)
		}

		signalsBySource = append(signalsBySource, signalsOf(perBarIndicatorValues))
	}

	return strategyBacktestDomain.ReplayOver(alignment, signalsBySource, settlements), nil
}

// refusalFor explains an unfinished replay, with an actionable message when the allowance ran out.
func (contractBacktestService *ContractBacktestService) refusalFor(
	replayContext context.Context, executionError error,
) error {
	// Waiting for a compartment is not cured by asking for less, so it is not blamed on the allowance.
	if errors.Is(executionError, domains.ErrIndicatorScriptCompartmentsBusy) {
		return executionError
	}
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return domains.BacktestTimeAllowanceSpent(contractBacktestService.replayTimeAllowance)
	}

	return executionError
}

// readTradingRules reads the symbol's specification and margin ladder first, since the allowed leverage depends on them.
func (contractBacktestService *ContractBacktestService) readTradingRules(
	executionContext context.Context, declaredSymbol string,
) (domains.ContractTradingRulesDomain, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(declaredSymbol)
	if symbolError != nil {
		return domains.ContractTradingRulesDomain{}, fmt.Errorf(
			"%w: %w", domains.ErrBacktestValidation, symbolError)
	}

	contractTradingSymbol, isRegistered, findSymbolError := contractBacktestService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findSymbolError != nil {
		return domains.ContractTradingRulesDomain{}, findSymbolError
	}

	maintenanceMarginTiers, findTiersError := contractBacktestService.
		contractMaintenanceMarginTierRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findTiersError != nil {
		return domains.ContractTradingRulesDomain{}, findTiersError
	}

	return domains.NewContractTradingRulesDomain(contractTradingSymbol, isRegistered, maintenanceMarginTiers)
}

// readReplayBars merges stored contract K candles into bars with funding and positioning aligned, exactly as indicator calculation does, returning the account view, the script view and the settlements.
func (contractBacktestService *ContractBacktestService) readReplayBars(
	executionContext context.Context, contractBacktestDomain domains.ContractBacktestDomain,
) (domains.ContractKCandleAlignmentDomain, []vo.ContractKCandleVo, []entities.ContractFundingRateSettlement, error) {
	kCandleContracts, findCandlesError := contractBacktestService.kCandleContractRepository.FindInRange(
		executionContext, contractBacktestDomain.KCandleQuery(), contractBacktestDomain.SourceCandleLimit())
	if findCandlesError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, findCandlesError
	}

	alignment, selectionError := contractBacktestDomain.SelectInput(kCandleContracts)
	if selectionError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, selectionError
	}

	settlements, findSettlementsError := contractBacktestService.contractFundingRateSettlementRepository.
		FindInRange(executionContext, alignment.SettlementQuery(), alignment.SettlementReadLimit())
	if findSettlementsError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, findSettlementsError
	}

	leadInSettlement, hasLeadIn, findLeadInError := contractBacktestService.
		contractFundingRateSettlementRepository.
		FindLatestBefore(executionContext, contractBacktestDomain.Symbol(), alignment.SettlementLeadInCutoff())
	if findLeadInError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, findLeadInError
	}
	if hasLeadIn {
		settlements = append(settlements, leadInSettlement)
	}

	statistics, findStatisticsError := contractBacktestService.contractPositionStatisticRepository.
		FindInRange(executionContext, alignment.StatisticQuery(), alignment.StatisticReadLimit())
	if findStatisticsError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, findStatisticsError
	}

	return alignment, alignment.Aligning(settlements, statistics), settlements, nil
}
