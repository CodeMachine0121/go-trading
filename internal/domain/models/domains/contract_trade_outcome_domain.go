package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	rMultipleUnavailableNoStopLoss           = "noStopLoss"
	outcomeUnavailableNotOpen                = "notOpen"
	outcomeUnavailableNoLatestPrice          = "noLatestPrice"
	outcomeUnavailableNoTradingSpecification = "noTradingSpecification"
	outcomeUnavailableNotComputed            = "notComputed"
)

// ContractTradeOutcomeDomain puts a trade's fills, plan and market facts together into what it came to.
type ContractTradeOutcomeDomain struct {
	record          entities.ContractTradeRecord
	ledger          ContractTradeLedgerDomain
	direction       vo.PositionDirectionVo
	facts           vo.ContractTradeMarketFactsVo
	tradingRules    ContractTradingRulesDomain
	rulesRequested  bool
	hasTradingRules bool
}

func NewContractTradeOutcomeDomain(
	recordDomain ContractTradeRecordDomain, facts vo.ContractTradeMarketFactsVo,
) ContractTradeOutcomeDomain {
	return ContractTradeOutcomeDomain{
		record:    recordDomain.record,
		ledger:    recordDomain.ledger,
		direction: recordDomain.Direction(),
		facts:     facts,
	}
}

// WithTradingRules asks for a liquidation estimate; hasTradingRules false means the symbol has no specification yet.
func (outcomeDomain ContractTradeOutcomeDomain) WithTradingRules(
	tradingRules ContractTradingRulesDomain, hasTradingRules bool,
) ContractTradeOutcomeDomain {
	outcomeDomain.tradingRules = tradingRules
	outcomeDomain.rulesRequested = true
	outcomeDomain.hasTradingRules = hasTradingRules

	return outcomeDomain
}

func (outcomeDomain ContractTradeOutcomeDomain) Outcome() dto.ContractTradeOutcomeDto {
	ledger := outcomeDomain.ledger
	grossProfit := ledger.GrossProfit(outcomeDomain.direction)
	totalFee := ledger.TotalFee()
	funding := NewContractTradeFundingDomain(ledger, outcomeDomain.direction).FundingFor(outcomeDomain.facts)

	netProfit := grossProfit.Sub(totalFee)
	if funding.Available {
		netProfit = netProfit.Add(funding.Amount)
	}

	plannedRisk := NewPlannedRiskDomain(
		ledger.AverageEntryPrice(), ledger.EnteredQuantity(), outcomeDomain.record.PlannedStopLossPrice)

	outcomeDto := dto.ContractTradeOutcomeDto{
		GrossProfit:              grossProfit,
		TotalFee:                 totalFee,
		FeeRateMissing:           ledger.FeeRateMissing(),
		Funding:                  funding,
		NetProfit:                netProfit,
		NetProfitExcludesFunding: !funding.Available,
		PlannedRisk:              plannedRisk.Value(),
		RMultiple:                plannedRisk.RMultipleOf(netProfit),
		Excursion: NewContractTradeExcursionDomain(ledger, outcomeDomain.direction, plannedRisk).
			ExcursionFor(outcomeDomain.facts),
		FloatingProfit:          outcomeDomain.floatingProfit(),
		LiquidationPrice:        outcomeDomain.liquidationPrice(),
		EntrySlippagePercentage: outcomeDomain.entrySlippagePercentage(),
	}

	if !plannedRisk.Value().Valid {
		outcomeDto.RMultipleUnavailableReason = rMultipleUnavailableNoStopLoss
	}

	// Only a finished trade has a share captured; while held, the realised part is not the trade's result.
	isHeld := outcomeDomain.record.Status == string(vo.ContractTradeStatusOpen)
	if !isHeld && outcomeDto.Excursion.Available && outcomeDto.Excursion.FavorableProfit.IsPositive() {
		profitCaptureRate := grossProfit.Div(outcomeDto.Excursion.FavorableProfit).InexactFloat64()
		outcomeDto.ProfitCaptureRate = &profitCaptureRate
	}

	return outcomeDto
}

func (outcomeDomain ContractTradeOutcomeDomain) floatingProfit() dto.ContractTradeFloatingDto {
	if outcomeDomain.record.Status != string(vo.ContractTradeStatusOpen) {
		return dto.ContractTradeFloatingDto{UnavailableReason: outcomeUnavailableNotOpen}
	}
	if !outcomeDomain.facts.HasLatestPrice {
		return dto.ContractTradeFloatingDto{UnavailableReason: outcomeUnavailableNoLatestPrice}
	}

	return dto.ContractTradeFloatingDto{
		Available: true,
		Price:     outcomeDomain.facts.LatestPrice,
		Amount:    outcomeDomain.ledger.OpenProfitAt(outcomeDomain.direction, outcomeDomain.facts.LatestPrice),
	}
}

// liquidationPrice reuses the replay's isolated-margin formula on what is still held, so the journal and the replay never disagree.
func (outcomeDomain ContractTradeOutcomeDomain) liquidationPrice() dto.ContractTradeLiquidationDto {
	if outcomeDomain.record.Status != string(vo.ContractTradeStatusOpen) {
		return dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNotOpen}
	}
	if !outcomeDomain.rulesRequested {
		return dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNotComputed}
	}
	if !outcomeDomain.hasTradingRules {
		return dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNoTradingSpecification}
	}

	averageEntryPrice := outcomeDomain.ledger.AverageEntryPrice()
	position := outcomeDomain.ledger.Position()
	notional := averageEntryPrice.Mul(position)

	openPosition := ContractBacktestPositionDomain{
		direction:       outcomeDomain.direction,
		entryPrice:      averageEntryPrice,
		quantity:        position,
		leverage:        outcomeDomain.record.Leverage,
		margin:          notional.Div(outcomeDomain.record.Leverage),
		maintenanceTier: outcomeDomain.tradingRules.TierFor(notional),
	}

	liquidationPrice := openPosition.LiquidationPrice()
	if !liquidationPrice.IsPositive() {
		return dto.ContractTradeLiquidationDto{Available: true, CannotBeLiquidated: true}
	}

	return dto.ContractTradeLiquidationDto{
		Available: true,
		Price:     outcomeDomain.tradingRules.RoundedToTick(liquidationPrice),
	}
}

// entrySlippagePercentage is positive when the fill was worse than the bot's reference price: paying more on a long, receiving less on a short.
func (outcomeDomain ContractTradeOutcomeDomain) entrySlippagePercentage() *float64 {
	referencePrice := outcomeDomain.record.SourceReferencePrice
	if !referencePrice.Valid || !referencePrice.Decimal.IsPositive() {
		return nil
	}

	difference := outcomeDomain.ledger.AverageEntryPrice().Sub(referencePrice.Decimal)
	if outcomeDomain.direction == vo.PositionDirectionShort {
		difference = difference.Neg()
	}

	slippagePercentage := difference.Div(referencePrice.Decimal).Mul(oneHundredPercent).InexactFloat64()

	return &slippagePercentage
}
