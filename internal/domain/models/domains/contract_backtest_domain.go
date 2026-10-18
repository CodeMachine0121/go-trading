package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractBacktestDomain validates a contract replay; everything shared with spot is validated by BacktestDomain so both give identical messages, and only leverage, trading mode, slippage and venue rules are read here.
type ContractBacktestDomain struct {
	backtest      BacktestDomain
	tradingMode   ContractTradingModeDomain
	positionTerms ContractPositionTermsDomain
	tradingRules  ContractTradingRulesDomain
}

func NewContractBacktestDomain(
	requestDto dto.ContractBacktestRequestDto,
	tradingRules ContractTradingRulesDomain,
	maxCandleCount int,
	now time.Time,
) (ContractBacktestDomain, error) {
	backtest, backtestError := NewBacktestDomain(requestDto.ToBacktestRequestDto(), maxCandleCount, now)
	if backtestError != nil {
		return ContractBacktestDomain{}, backtestError
	}

	if reachError := backtest.KCandleQuery().requireOrderFlowReach(backtest.Parameters()); reachError != nil {
		return ContractBacktestDomain{}, reachError
	}

	if !requestDto.MaintenanceMarginRate.IsZero() {
		return ContractBacktestDomain{}, BacktestValidationFailure(BacktestMaintenanceMarginRateField,
			"維持保證金率由這個合約標的的分級決定，重演時不能另外指定")
	}

	tradingMode, tradingModeError := NewContractTradingModeDomain(requestDto.TradingMode)
	if tradingModeError != nil {
		return ContractBacktestDomain{}, BacktestValidationFailure(
			BacktestTradingModeField, tradingModeError.Error())
	}

	leverage := requestDto.Leverage
	if leverage.IsZero() {
		leverage = oneWhole
	}
	if leverage.LessThan(oneWhole) {
		return ContractBacktestDomain{}, BacktestValidationFailure(
			BacktestLeverageField, ErrLeverageMultiplierBelowOne.Error())
	}
	if maximumLeverage, isLimited := tradingRules.MaximumLeverage(); isLimited &&
		leverage.GreaterThan(decimal.NewFromInt(int64(maximumLeverage))) {
		return ContractBacktestDomain{}, BacktestValidationFailure(BacktestLeverageField, fmt.Sprintf(
			"這個合約標的最高只能開 %d 倍槓桿", maximumLeverage))
	}

	slippage, slippageError := NewBacktestSlippageDomain(requestDto.SlippagePercentage)
	if slippageError != nil {
		return ContractBacktestDomain{}, BacktestValidationFailure(
			BacktestSlippageField, slippageError.Error())
	}

	positionTerms := NewContractPositionTermsDomain(backtest.positionTerms, leverage, slippage, tradingRules)
	if positionTerms.NeverOpensAnything() {
		return ContractBacktestDomain{}, BacktestValidationFailure(
			BacktestPositionSizingValueField,
			"這個百分比連同它照名目收的進場成本付不起，每一次開倉都會被跳過，"+
				"這次重演一筆交易都不會有。要押滿請改用全押——它會自己留出手續費")
	}

	return ContractBacktestDomain{
		backtest:      backtest,
		tradingMode:   tradingMode,
		positionTerms: positionTerms,
		tradingRules:  tradingRules,
	}, nil
}

func (contractBacktestDomain ContractBacktestDomain) Symbol() string {
	return contractBacktestDomain.backtest.Symbol()
}

func (contractBacktestDomain ContractBacktestDomain) Parameters() StrategyScriptParametersDomain {
	return contractBacktestDomain.backtest.Parameters()
}

func (contractBacktestDomain ContractBacktestDomain) ResultType() IndicatorResultTypeDomain {
	return contractBacktestDomain.backtest.ResultType()
}

// KCandleQuery is the same stretch a spot replay of the request would read.
func (contractBacktestDomain ContractBacktestDomain) KCandleQuery() KCandleQueryDomain {
	return contractBacktestDomain.backtest.KCandleQuery()
}

func (contractBacktestDomain ContractBacktestDomain) SourceCandleLimit() int {
	return contractBacktestDomain.backtest.SourceCandleLimit()
}

// SelectInput merges contract K candles into finished bars, earliest first, returning the alignment for funding and positioning; fewer than two bars is refused.
func (contractBacktestDomain ContractBacktestDomain) SelectInput(
	kCandleContracts []entities.KCandleContract,
) (ContractKCandleAlignmentDomain, error) {
	mergedBuckets := NewKCandleContractSeriesDomain(
		contractBacktestDomain.backtest.symbol,
		contractBacktestDomain.backtest.interval,
		kCandleContracts,
	).ToDto().KCandles

	finishedBuckets := make([]dto.KCandleContractDto, 0, len(mergedBuckets))
	for _, bucket := range mergedBuckets {
		if bucket.OpenTime.Before(contractBacktestDomain.backtest.readCutoff) {
			finishedBuckets = append(finishedBuckets, bucket)
		}
	}

	tradedBuckets := finishedBuckets[contractBacktestDomain.tradeStartIndex(finishedBuckets):]
	if len(tradedBuckets) < minimumBacktestKCandleCount {
		return ContractKCandleAlignmentDomain{}, notEnoughKCandlesForBacktest(len(tradedBuckets))
	}

	// Refuse a split lacking a bar on either side before any script runs.
	if _, splitError := contractBacktestDomain.splitIndexOf(tradedBuckets); splitError != nil {
		return ContractKCandleAlignmentDomain{}, splitError
	}

	return newContractKCandleAlignmentDomain(contractBacktestDomain.backtest.interval, finishedBuckets), nil
}

// FinishedAlignmentAt merges the read contract K candles into another interval, keeping only finished buckets.
func (contractBacktestDomain ContractBacktestDomain) FinishedAlignmentAt(
	interval AggregationIntervalDomain, kCandleContracts []entities.KCandleContract,
) (ContractKCandleAlignmentDomain, bool) {
	mergedBuckets := NewKCandleContractSeriesDomain(
		contractBacktestDomain.backtest.symbol, interval, kCandleContracts).ToDto().KCandles

	finishedBuckets := make([]dto.KCandleContractDto, 0, len(mergedBuckets))
	for _, bucket := range mergedBuckets {
		if contractBacktestDomain.backtest.isFinishedBy(interval, bucket.OpenTime) {
			finishedBuckets = append(finishedBuckets, bucket)
		}
	}

	if len(finishedBuckets) == 0 {
		return ContractKCandleAlignmentDomain{}, false
	}

	return newContractKCandleAlignmentDomain(interval, finishedBuckets), true
}

func (contractBacktestDomain ContractBacktestDomain) tradeStartIndex(buckets []dto.KCandleContractDto) int {
	return contractBacktestDomain.backtest.tradeStartIndex(contractBacktestDomain.openTimesOf(buckets))
}

// splitIndexOf returns the full length for an unsplit replay.
func (contractBacktestDomain ContractBacktestDomain) splitIndexOf(tradedBuckets []dto.KCandleContractDto) (int, error) {
	if !contractBacktestDomain.backtest.segments.IsSplit() {
		return len(tradedBuckets), nil
	}

	return contractBacktestDomain.backtest.segments.SplitIndex(contractBacktestDomain.openTimesOf(tradedBuckets))
}

func (contractBacktestDomain ContractBacktestDomain) openTimesOf(buckets []dto.KCandleContractDto) []time.Time {
	openTimes := make([]time.Time, 0, len(buckets))
	for _, bucket := range buckets {
		openTimes = append(openTimes, bucket.OpenTime.UTC())
	}

	return openTimes
}

// ReplayOver replays the bars with funding settlements and, when split, each part again from the initial capital and flat.
// conflictedFlags marks bars where both of a trading strategy's trees held; strategy script replays pass nil.
func (contractBacktestDomain ContractBacktestDomain) ReplayOver(
	alignment ContractKCandleAlignmentDomain,
	signals []SignalDomain,
	settlements []entities.ContractFundingRateSettlement,
	conflictedFlags []bool,
) dto.ContractBacktestResultDto {
	buckets := alignment.buckets
	if conflictedFlags == nil {
		conflictedFlags = make([]bool, len(buckets))
	}

	// The warm-up bars shaped the signals but are never traded, so the report starts at the stretch.
	tradeStartIndex := contractBacktestDomain.tradeStartIndex(buckets)
	buckets = buckets[tradeStartIndex:]
	signals = signals[tradeStartIndex:]
	conflictedFlags = conflictedFlags[tradeStartIndex:]

	replayed := func(
		partBuckets []dto.KCandleContractDto, partSignals []SignalDomain, partConflictedFlags []bool,
	) dto.ContractBacktestResultDto {
		resultDto := NewContractBacktestSimulationDomain(
			contractBacktestDomain.backtest.initialCapital,
			contractBacktestDomain.positionTerms,
			contractBacktestDomain.tradingMode,
			contractBacktestDomain.backtest.fillTiming,
			contractBacktestDomain.tradingRules,
			contractBacktestDomain.backtest.interval,
			partBuckets,
			partSignals,
			settlements,
		).ToDto()

		resultDto.Symbol = contractBacktestDomain.backtest.symbol
		resultDto.Interval = string(contractBacktestDomain.backtest.interval.Value())
		for _, isConflicted := range partConflictedFlags {
			if isConflicted {
				resultDto.Summary.ConflictedCandleCount++
			}
		}

		return resultDto
	}

	wholeResultDto := replayed(buckets, signals, conflictedFlags)
	if !contractBacktestDomain.backtest.segments.IsSplit() {
		return wholeResultDto
	}

	// Already checked in SelectInput; if it fails anyway, the whole replay still stands.
	splitIndex, splitError := contractBacktestDomain.splitIndexOf(buckets)
	if splitError != nil {
		return wholeResultDto
	}

	inSampleResultDto := replayed(buckets[:splitIndex], signals[:splitIndex], conflictedFlags[:splitIndex])
	validationResultDto := replayed(buckets[splitIndex:], signals[splitIndex:], conflictedFlags[splitIndex:])
	wholeResultDto.ValidationStartTime = contractBacktestDomain.backtest.segments.ValidationStartTime()
	wholeResultDto.InSample = &inSampleResultDto
	wholeResultDto.Validation = &validationResultDto

	return wholeResultDto
}
