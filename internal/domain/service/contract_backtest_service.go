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

// ContractBacktestService is the application layer's only entry point for replaying a
// contract strategy script, or a contract trading strategy, on an isolated contract
// account over a stretch of market that has already happened.
//
// It orchestrates and nothing more: which symbol's rules apply, which bars to read,
// which funding and positioning to line up beside them, and in what order the script
// runs. Every rule lives in the domain models it hands those to.
type ContractBacktestService struct {
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	contractPositionStatisticRepository     domaininterface.IContractPositionStatisticRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository
	contractIndicatorScriptProxy            domaininterface.IContractIndicatorScriptProxy
	clockProxy                              domaininterface.IClockProxy
	maxCandleCount                          int
	// replayTimeAllowance is how long one whole replay may run its scripts for, every
	// source together — the same allowance the spot replay has.
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

// RunContractBacktest replays one contract strategy script: the script runs once per
// finished bar and sees every bar from the first up to the one it stands on, and the
// account trades on what it says. Nothing is stored.
func (contractBacktestService *ContractBacktestService) RunContractBacktest(
	executionContext context.Context, requestDto dto.ContractBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	tradingRules, rulesError := contractBacktestService.readTradingRules(executionContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, rulesError
	}

	contractBacktestDomain, validationError := domains.NewContractBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	alignment, contractKCandles, settlements, readError := contractBacktestService.readReplayBars(
		executionContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, readError
	}

	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, contractBacktestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

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

// RunContractTradingStrategyBacktest replays a whole contract trading strategy over
// the same kind of stretch: every source runs its own script over the same bars, the
// two condition trees turn each bar's several opinions into one, and the account trades
// on that by the trading strategy's own trading mode.
//
// The bars are read once and every source runs over that one batch, for the reason
// the spot replay does: reading per source would be up to ten chances for two sources
// to replay slightly different stretches.
func (contractBacktestService *ContractBacktestService) RunContractTradingStrategyBacktest(
	executionContext context.Context, requestDto dto.ContractTradingStrategyBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	tradingRules, rulesError := contractBacktestService.readTradingRules(executionContext, requestDto.Symbol)
	if rulesError != nil {
		return dto.ContractBacktestResultDto{}, rulesError
	}

	strategyBacktestDomain, validationError := domains.NewContractTradingStrategyBacktestDomain(
		requestDto, tradingRules, contractBacktestService.maxCandleCount, contractBacktestService.clockProxy.Now())
	if validationError != nil {
		return dto.ContractBacktestResultDto{}, validationError
	}

	contractBacktestDomain := strategyBacktestDomain.ContractBacktest()
	alignment, contractKCandles, settlements, readError := contractBacktestService.readReplayBars(
		executionContext, contractBacktestDomain)
	if readError != nil {
		return dto.ContractBacktestResultDto{}, readError
	}

	replayContext, stopReplaying := context.WithTimeoutCause(
		executionContext, contractBacktestService.replayTimeAllowance, errReplayTimeAllowanceSpent)
	defer stopReplaying()

	signalsBySource := make([][]domains.SignalDomain, 0, strategyBacktestDomain.SourceCount())
	for sourceIndex := range strategyBacktestDomain.SourceCount() {
		perBarIndicatorValues, executionError := contractBacktestService.contractIndicatorScriptProxy.ExecuteForEachCandle(
			replayContext,
			strategyBacktestDomain.SourceScript(sourceIndex),
			contractBacktestDomain.ResultType(),
			contractKCandles,
			strategyBacktestDomain.SourceParameters(sourceIndex))
		// One source failing ends the whole replay: half a replay would answer the
		// conditions against signals that are simply absent.
		if executionError != nil {
			return dto.ContractBacktestResultDto{}, contractBacktestService.refusalFor(replayContext, executionError)
		}

		signalsBySource = append(signalsBySource, signalsOf(perBarIndicatorValues))
	}

	return strategyBacktestDomain.ReplayOver(alignment, signalsBySource, settlements), nil
}

// refusalFor is what a replay says when its scripts stopped: the allowance, in words
// a person can act on, when that is what ran out; otherwise whatever stopped them.
func (contractBacktestService *ContractBacktestService) refusalFor(
	replayContext context.Context, executionError error,
) error {
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return domains.BacktestTimeAllowanceSpent(contractBacktestService.replayTimeAllowance)
	}

	return executionError
}

// readTradingRules reads the venue's rules for the symbol a replay names: its trading
// specification and its maintenance margin ladder. They are read before anything else,
// because the leverage a request may ask for is one of them.
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

// readReplayBars reads the stored contract K candles of the stretch, merges them into
// finished bars and lines each bar's funding and positioning up beside it — exactly as
// a contract indicator calculation does, so a script sees the same bar either way.
//
// It hands back the bars as the account reads them (the alignment), the same bars as
// the script reads them, and the funding settlements that fall among them.
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
