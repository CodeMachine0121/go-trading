package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractBacktestSimulationDomain replays a contract account bar by bar. Like its spot
// counterpart it is pure arithmetic — bars, opinions and settlements in, a report card
// out — and it is a model of its own rather than a mode of the spot one, because the
// two change for different reasons: the spot one when spot trading rules change, this
// one when the venue's do.
type ContractBacktestSimulationDomain struct {
	initialCapital decimal.Decimal
	positionTerms  ContractPositionTermsDomain
	tradingMode    ContractTradingModeDomain
	fillTiming     BacktestFillTimingDomain
	tradingRules   ContractTradingRulesDomain
	interval       AggregationIntervalDomain
	buckets        []dto.KCandleContractDto
	// signals holds exactly one opinion per bar: the nth belongs to the nth bar.
	signals     []SignalDomain
	settlements []entities.ContractFundingRateSettlement
}

func NewContractBacktestSimulationDomain(
	initialCapital decimal.Decimal,
	positionTerms ContractPositionTermsDomain,
	tradingMode ContractTradingModeDomain,
	fillTiming BacktestFillTimingDomain,
	tradingRules ContractTradingRulesDomain,
	interval AggregationIntervalDomain,
	buckets []dto.KCandleContractDto,
	signals []SignalDomain,
	settlements []entities.ContractFundingRateSettlement,
) ContractBacktestSimulationDomain {
	return ContractBacktestSimulationDomain{
		initialCapital: initialCapital,
		positionTerms:  positionTerms,
		tradingMode:    tradingMode,
		fillTiming:     fillTiming,
		tradingRules:   tradingRules,
		interval:       interval,
		buckets:        buckets,
		signals:        signals,
		settlements:    settlements,
	}
}

// ToDto walks the bars once. Within a bar the order is the rule, and it is chosen so
// that nothing the high and low cannot tell apart is ever read in the strategy's
// favour: the bar's funding first, then the exit levels, then the bar's own signal,
// filled at its close.
func (simulationDomain ContractBacktestSimulationDomain) ToDto() dto.ContractBacktestResultDto {
	account := NewContractBacktestAccountDomain(simulationDomain.initialCapital, simulationDomain.positionTerms)
	equityCurve := NewBacktestEquityCurveDomain(simulationDomain.initialCapital)

	earliestFirstSettlements := slices.Clone(simulationDomain.settlements)
	slices.SortFunc(earliestFirstSettlements, func(former, latter entities.ContractFundingRateSettlement) int {
		return former.SettlementTime.Compare(latter.SettlementTime)
	})
	nextSettlement := 0

	for bucketIndex, bucket := range simulationDomain.buckets {
		bucketStart := bucket.OpenTime.UTC()
		bucketEnd := bucketStart.Add(simulationDomain.interval.duration)

		for nextSettlement < len(earliestFirstSettlements) &&
			earliestFirstSettlements[nextSettlement].SettlementTime.Before(bucketStart) {
			nextSettlement++
		}
		settlementsInBucket := nextSettlement
		for settlementsInBucket < len(earliestFirstSettlements) &&
			earliestFirstSettlements[settlementsInBucket].SettlementTime.Before(bucketEnd) {
			settlementsInBucket++
		}

		settlementsOfBucket := earliestFirstSettlements[nextSettlement:settlementsInBucket]
		nextSettlement = settlementsInBucket

		// Filling at the next open, the previous bar's opinion is carried out at this
		// bar's open. A settlement stamped exactly at that open happened before the
		// fill, so only the position carried into the bar pays it; the bar's later
		// settlements are paid by whatever is held after the fill. The exit levels are
		// asked last, the position just opened included — its bar's high and low
		// happen after it was filled. The last bar's opinion is never filled.
		if simulationDomain.fillTiming.FillsAtNextOpen() {
			settledAtOpen := 0
			for settledAtOpen < len(settlementsOfBucket) &&
				!settlementsOfBucket[settledAtOpen].SettlementTime.After(bucketStart) {
				settledAtOpen++
			}

			account.SettleFundingWithin(bucket, settlementsOfBucket[:settledAtOpen])
			if bucketIndex > 0 {
				account.Apply(
					simulationDomain.tradingMode.TargetFor(simulationDomain.signals[bucketIndex-1]),
					bucketStart, bucket.Open)
			}
			account.SettleFundingWithin(bucket, settlementsOfBucket[settledAtOpen:])
			account.ApplyExitLevels(bucket, bucketStart)
			equityCurve.Record(bucketStart, account.EquityAt(bucket.Close))

			continue
		}

		account.SettleFundingWithin(bucket, settlementsOfBucket)
		account.ApplyExitLevels(bucket, bucketStart)
		account.Apply(
			simulationDomain.tradingMode.TargetFor(simulationDomain.signals[bucketIndex]),
			bucketStart, bucket.Close)
		equityCurve.Record(bucketStart, account.EquityAt(bucket.Close))
	}

	summaryDto := account.SummaryDto()
	summaryDto.InitialCapital = simulationDomain.initialCapital
	summaryDto.FinalEquity = equityCurve.FinalEquity()
	summaryDto.TotalReturnRate = equityCurve.TotalReturnRate()
	summaryDto.MaximumDrawdown = equityCurve.MaximumDrawdown()
	summaryDto.MaintenanceMarginBasis = simulationDomain.tradingRules.MaintenanceMarginBasisDto()

	equityPointDtos := equityCurve.PointDtos()

	return dto.ContractBacktestResultDto{
		TradingMode:     string(simulationDomain.tradingMode.Value()),
		FillTiming:      string(simulationDomain.fillTiming.Value()),
		Leverage:        simulationDomain.positionTerms.leverage,
		StartTime:       equityPointDtos[0].OpenTime,
		EndTime:         equityPointDtos[len(equityPointDtos)-1].OpenTime,
		UsedCandleCount: len(simulationDomain.buckets),
		Summary:         summaryDto,
		ClosedTrades:    account.ClosedTradeDtos(),
		EquityCurve:     equityPointDtos,
	}
}
