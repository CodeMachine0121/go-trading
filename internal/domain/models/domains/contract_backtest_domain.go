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

	return newContractKCandleAlignmentDomain(contractBacktestDomain.backtest.interval, finishedBuckets), nil
}

// ReplayOver walks the account over the bars the alignment holds, one opinion per bar,
// paying and receiving the funding settlements that fall inside them.
func (contractBacktestDomain ContractBacktestDomain) ReplayOver(
	alignment ContractKCandleAlignmentDomain,
	signals []SignalDomain,
	settlements []entities.ContractFundingRateSettlement,
) dto.ContractBacktestResultDto {
	resultDto := NewContractBacktestSimulationDomain(
		contractBacktestDomain.backtest.initialCapital,
		contractBacktestDomain.positionTerms,
		contractBacktestDomain.tradingMode,
		contractBacktestDomain.tradingRules,
		contractBacktestDomain.backtest.interval,
		alignment.buckets,
		signals,
		settlements,
	).ToDto()

	resultDto.Symbol = contractBacktestDomain.backtest.symbol
	resultDto.Interval = string(contractBacktestDomain.backtest.interval.Value())

	return resultDto
}
