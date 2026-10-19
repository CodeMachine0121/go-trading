package service

import (
	"context"
	"fmt"

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
	contractOrderFlowService                *ContractOrderFlowService
	clockProxy                              domaininterface.IClockProxy
	maxCandleCount                          int
	defaultWarmupBarCount                   int
}

func NewContractBacktestService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	contractPositionStatisticRepository domaininterface.IContractPositionStatisticRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository,
	contractIndicatorScriptProxy domaininterface.IContractIndicatorScriptProxy,
	contractOrderFlowService *ContractOrderFlowService,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
	defaultWarmupBarCount int,
) *ContractBacktestService {
	return &ContractBacktestService{
		kCandleContractRepository:               kCandleContractRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		contractPositionStatisticRepository:     contractPositionStatisticRepository,
		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository: contractMaintenanceMarginTierRepository,
		contractIndicatorScriptProxy:            contractIndicatorScriptProxy,
		contractOrderFlowService:                contractOrderFlowService,
		clockProxy:                              clockProxy,
		maxCandleCount:                          maxCandleCount,
		defaultWarmupBarCount:                   defaultWarmupBarCount,
	}
}

// ValidateContractBacktest judges a request exactly as a replay would, venue rules included, before reading any bar.
func (contractBacktestService *ContractBacktestService) ValidateContractBacktest(
	executionContext context.Context, requestDto dto.ContractBacktestRequestDto,
) error {
	tradingRules, rulesError := contractBacktestService.readTradingRules(executionContext, requestDto.Symbol)
	if rulesError != nil {
		return rulesError
	}
	requestDto = requestDto.WarmingUpByDefault(contractBacktestService.defaultWarmupBarCount)
	_, validationError := domains.NewContractBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())

	return validationError
}

func (contractBacktestService *ContractBacktestService) ValidateContractTradingStrategyBacktest(
	executionContext context.Context, requestDto dto.ContractTradingStrategyBacktestRequestDto,
) error {
	tradingRules, rulesError := contractBacktestService.readTradingRules(executionContext, requestDto.Symbol)
	if rulesError != nil {
		return rulesError
	}
	requestDto = requestDto.WarmingUpByDefault(contractBacktestService.defaultWarmupBarCount)
	_, validationError := domains.NewContractTradingStrategyBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())

	return validationError
}

// RunContractBacktest replays one contract script once per finished bar, each run seeing the bars up to its own within the declared look-back; nothing is stored.
func (contractBacktestService *ContractBacktestService) RunContractBacktest(
	executionContext context.Context,
	requestDto dto.ContractBacktestRequestDto,
	progress domaininterface.IReplayProgressSink,
) (dto.ContractBacktestResultDto, error) {
	allowance := domains.NewReplayAllowanceDomain(requestDto.TimeAllowance)
	replayContext, stopReplaying := allowance.Bound(executionContext)
	defer stopReplaying()

	tradingRules, rulesError := contractBacktestService.readTradingRules(replayContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, rulesError)
	}

	requestDto = requestDto.WarmingUpByDefault(contractBacktestService.defaultWarmupBarCount)
	contractBacktestDomain, validationError := domains.NewContractBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	alignment, alignedBars, settlements, _, _, readError := contractBacktestService.readReplayBars(
		replayContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, readError)
	}

	return contractBacktestService.replayContractScript(
		replayContext, allowance, requestDto.Script, contractBacktestDomain, alignment, alignedBars, settlements, progress)
}

// RunContractBacktestOn replays one contract script on market data prepared by the caller instead of the stored minute candles; position statistics read as absent.
func (contractBacktestService *ContractBacktestService) RunContractBacktestOn(
	executionContext context.Context,
	requestDto dto.ContractBacktestRequestDto,
	replayInput domains.ContractReplayInputDomain,
	progress domaininterface.IReplayProgressSink,
) (dto.ContractBacktestResultDto, error) {
	allowance := domains.NewReplayAllowanceDomain(requestDto.TimeAllowance)
	replayContext, stopReplaying := allowance.Bound(executionContext)
	defer stopReplaying()

	tradingRules, rulesError := replayInput.TradingRules()
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, rulesError
	}

	requestDto = requestDto.WarmingUpByDefault(contractBacktestService.defaultWarmupBarCount)
	contractBacktestDomain, validationError := domains.NewContractBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	alignment, selectionError := contractBacktestDomain.SelectInput(
		replayInput.CandlesIn(contractBacktestDomain.KCandleQuery()))
	if selectionError != nil {
		return dto.ContractBacktestResultDto{}, selectionError
	}

	settlements := replayInput.SettlementsIn(alignment.SettlementQuery())
	if leadInSettlement, hasLeadIn := replayInput.LatestSettlementBefore(alignment.SettlementLeadInCutoff()); hasLeadIn {
		settlements = append(settlements, leadInSettlement)
	}

	return contractBacktestService.replayContractScript(
		replayContext, allowance, requestDto.Script, contractBacktestDomain, alignment,
		alignment.Aligning(settlements, nil), settlements, progress)
}

// replayContractScript is the replay itself once bars are read, shared by stored and prepared market data.
func (contractBacktestService *ContractBacktestService) replayContractScript(
	replayContext context.Context,
	allowance domains.ReplayAllowanceDomain,
	script string,
	contractBacktestDomain domains.ContractBacktestDomain,
	alignment domains.ContractKCandleAlignmentDomain,
	alignedBars []vo.ContractKCandleVo,
	settlements []entities.ContractFundingRateSettlement,
	progress domaininterface.IReplayProgressSink,
) (dto.ContractBacktestResultDto, error) {
	contractKCandles, attachError := contractBacktestService.contractOrderFlowService.AttachFootprints(
		replayContext, alignment, alignedBars, contractBacktestDomain.Parameters(), 0)
	if attachError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, attachError)
	}

	perBarIndicatorValues, executionError := contractBacktestService.contractIndicatorScriptProxy.ExecuteForEachCandle(
		replayContext,
		script,
		contractBacktestDomain.ResultType(),
		contractKCandles,
		contractBacktestDomain.Parameters(),
		progress)
	if executionError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, executionError)
	}

	return contractBacktestDomain.ReplayOver(
		alignment, signalsOf(perBarIndicatorValues), settlements, nil), nil
}

// RunContractTradingStrategyBacktest replays a contract trading strategy by its trading mode; bars are read once so every source sees the identical stretch.
func (contractBacktestService *ContractBacktestService) RunContractTradingStrategyBacktest(
	executionContext context.Context,
	requestDto dto.ContractTradingStrategyBacktestRequestDto,
	progress domaininterface.IReplayProgressSink,
) (dto.ContractBacktestResultDto, error) {
	allowance := domains.NewReplayAllowanceDomain(requestDto.TimeAllowance)
	replayContext, stopReplaying := allowance.Bound(executionContext)
	defer stopReplaying()

	tradingRules, rulesError := contractBacktestService.readTradingRules(replayContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, rulesError)
	}

	requestDto = requestDto.WarmingUpByDefault(contractBacktestService.defaultWarmupBarCount)
	strategyBacktestDomain, validationError := domains.NewContractTradingStrategyBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	contractBacktestDomain := strategyBacktestDomain.ContractBacktest()
	alignment, contractKCandles, settlements, statistics, kCandleContracts, readError :=
		contractBacktestService.readReplayBars(replayContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, readError)
	}

	sourceCount := strategyBacktestDomain.SourceCount()
	signalsBySource := make([][]domains.SignalDomain, 0, sourceCount)
	for sourceIndex := range sourceCount {
		sourceAlignment, sourceKCandles := alignment, contractKCandles
		if !strategyBacktestDomain.SourceIsAtPace(sourceIndex) {
			coarserAlignment, hasFinishedBar := strategyBacktestDomain.SourceAlignment(sourceIndex, kCandleContracts)
			if !hasFinishedBar {
				signalsBySource = append(signalsBySource, strategyBacktestDomain.SignalsAtPace(
					sourceIndex, coarserAlignment, nil, alignment))

				continue
			}
			sourceAlignment, sourceKCandles = coarserAlignment, coarserAlignment.Aligning(settlements, statistics)
		}

		// Each source gets footprints at its own price step; sources sharing a step share remembered days.
		sourceBars, attachError := contractBacktestService.contractOrderFlowService.AttachFootprints(
			replayContext, sourceAlignment, sourceKCandles, strategyBacktestDomain.SourceParameters(sourceIndex), 0)
		if attachError != nil {
			return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, attachError)
		}

		perBarIndicatorValues, executionError := contractBacktestService.contractIndicatorScriptProxy.ExecuteForEachCandle(
			replayContext,
			strategyBacktestDomain.SourceScript(sourceIndex),
			contractBacktestDomain.ResultType(),
			sourceBars,
			strategyBacktestDomain.SourceParameters(sourceIndex),
			newSourceReplayProgress(progress, sourceIndex, sourceCount))
		// One failing source ends the replay; missing signals would silently make conditions false.
		if executionError != nil {
			return dto.ContractBacktestResultDto{}, allowance.RefusalFor(replayContext, executionError)
		}

		sourceSignals := signalsOf(perBarIndicatorValues)
		if !strategyBacktestDomain.SourceIsAtPace(sourceIndex) {
			sourceSignals = strategyBacktestDomain.SignalsAtPace(sourceIndex, sourceAlignment, sourceSignals, alignment)
		}

		signalsBySource = append(signalsBySource, sourceSignals)
	}

	return strategyBacktestDomain.ReplayOver(alignment, signalsBySource, settlements), nil
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

// readReplayBars merges stored contract K candles into bars with funding and positioning aligned, exactly as indicator calculation does, returning the account view, the script view, the settlements and statistics, and the raw candles coarser sources merge again.
func (contractBacktestService *ContractBacktestService) readReplayBars(
	executionContext context.Context, contractBacktestDomain domains.ContractBacktestDomain,
) (
	domains.ContractKCandleAlignmentDomain,
	[]vo.ContractKCandleVo,
	[]entities.ContractFundingRateSettlement,
	[]entities.ContractPositionStatistic,
	[]entities.KCandleContract,
	error,
) {
	kCandleContracts, findCandlesError := contractBacktestService.kCandleContractRepository.FindInRange(
		executionContext, contractBacktestDomain.KCandleQuery(), contractBacktestDomain.SourceCandleLimit())
	if findCandlesError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, nil, nil, findCandlesError
	}

	alignment, selectionError := contractBacktestDomain.SelectInput(kCandleContracts)
	if selectionError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, nil, nil, selectionError
	}

	settlements, findSettlementsError := contractBacktestService.contractFundingRateSettlementRepository.
		FindInRange(executionContext, alignment.SettlementQuery(), alignment.SettlementReadLimit())
	if findSettlementsError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, nil, nil, findSettlementsError
	}

	leadInSettlement, hasLeadIn, findLeadInError := contractBacktestService.
		contractFundingRateSettlementRepository.
		FindLatestBefore(executionContext, contractBacktestDomain.Symbol(), alignment.SettlementLeadInCutoff())
	if findLeadInError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, nil, nil, findLeadInError
	}
	if hasLeadIn {
		settlements = append(settlements, leadInSettlement)
	}

	statistics, findStatisticsError := contractBacktestService.contractPositionStatisticRepository.
		FindInRange(executionContext, alignment.StatisticQuery(), alignment.StatisticReadLimit())
	if findStatisticsError != nil {
		return domains.ContractKCandleAlignmentDomain{}, nil, nil, nil, nil, findStatisticsError
	}

	return alignment, alignment.Aligning(settlements, statistics), settlements, statistics, kCandleContracts, nil
}
