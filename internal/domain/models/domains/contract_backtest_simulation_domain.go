package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractBacktestSimulationDomain replays a contract account bar by bar as pure arithmetic, kept separate from the spot simulation because it follows venue rules.
type ContractBacktestSimulationDomain struct {
	initialCapital decimal.Decimal
	positionTerms  ContractPositionTermsDomain
	tradingMode    ContractTradingModeDomain
	fillTiming     BacktestFillTimingDomain
	tradingRules   ContractTradingRulesDomain
	interval       AggregationIntervalDomain
	buckets        []dto.KCandleContractDto
	// signals holds exactly one opinion per bar.
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

// ToDto walks each bar as funding, then exit levels, then the bar's signal at its close, so ambiguous intrabar moves never favour the strategy.
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

		// Next-open fills: settlements at or before the open are paid by the carried position, then the previous bar's signal fills at the open, then later settlements and exit levels apply; the last bar's signal is never filled.
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
