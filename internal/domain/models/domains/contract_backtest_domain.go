package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractBacktestDomain is one replay of a contract strategy script on an isolated
// contract account, and every rule about what may be replayed.
//
// Everything a spot replay is also told — the stretch, the capital, the sizing, the
// exits, the costs, the knobs — is read by BacktestDomain, word for word, so that the
// same figure typed into either replay is refused with the same sentence. What is
// read here is only what a contract account adds: the leverage, the trading mode, the
// slippage, and the venue's rules for this symbol.
type ContractBacktestDomain struct {
	backtest      BacktestDomain
	tradingMode   ContractTradingModeDomain
	positionTerms ContractPositionTermsDomain
	tradingRules  ContractTradingRulesDomain
}

// NewContractBacktestDomain validates the request against the rules of a contract
// replay on a symbol traded by those rules.
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

// KCandleQuery is the stretch of contract K candles to read, exactly the stretch a spot
// replay of the same request would read.
func (contractBacktestDomain ContractBacktestDomain) KCandleQuery() KCandleQueryDomain {
	return contractBacktestDomain.backtest.KCandleQuery()
}

func (contractBacktestDomain ContractBacktestDomain) SourceCandleLimit() int {
	return contractBacktestDomain.backtest.SourceCandleLimit()
}

// SelectInput merges the contract K candles into finished bars, earliest first, and
// hands back the alignment that lines each bar's funding and positioning up for the
// script. Fewer than two bars is refused for the reason a spot replay refuses it.
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

	if len(finishedBuckets) < minimumBacktestKCandleCount {
		return ContractKCandleAlignmentDomain{}, notEnoughKCandlesForBacktest(len(finishedBuckets))
	}

	// A split replay needs a bar on either side of the validation start, refused before
	// a single script is run.
	if _, splitError := contractBacktestDomain.splitIndexOf(finishedBuckets); splitError != nil {
		return ContractKCandleAlignmentDomain{}, splitError
	}

	return newContractKCandleAlignmentDomain(contractBacktestDomain.backtest.interval, finishedBuckets), nil
}

// splitIndexOf is where these bars divide into the in-sample and the validation part,
// asked once when they are selected and once when they are replayed. An unsplit
// replay divides nowhere and answers the whole length.
func (contractBacktestDomain ContractBacktestDomain) splitIndexOf(buckets []dto.KCandleContractDto) (int, error) {
	if !contractBacktestDomain.backtest.segments.IsSplit() {
		return len(buckets), nil
	}

	openTimes := make([]time.Time, 0, len(buckets))
	for _, bucket := range buckets {
		openTimes = append(openTimes, bucket.OpenTime.UTC())
	}

	return contractBacktestDomain.backtest.segments.SplitIndex(openTimes)
}

// ReplayOver walks the account over the bars the alignment holds, one opinion per bar,
// paying and receiving the funding settlements that fall inside them — and for a split
// replay walks each part again on its own, from the initial capital and flat.
//
// conflictedFlags says, bar by bar, whether a trading strategy's two trees both held; a
// strategy script replay has no such thing and passes nil.
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

	// The bars were checked for one on either side when they were selected, so the split
	// cannot fail here; should it, the whole replay still stands on its own.
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
