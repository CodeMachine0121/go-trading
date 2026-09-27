package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SpotTradeOutcomeDomain puts a spot trade's buys, sells, plan and market facts together into what it came to; spot is only ever long.
type SpotTradeOutcomeDomain struct {
	record entities.SpotTradeRecord
	ledger TradeLedgerDomain
	facts  vo.SpotTradeMarketFactsVo
}

func NewSpotTradeOutcomeDomain(recordDomain SpotTradeRecordDomain, facts vo.SpotTradeMarketFactsVo) SpotTradeOutcomeDomain {
	return SpotTradeOutcomeDomain{record: recordDomain.record, ledger: recordDomain.Ledger(), facts: facts}
}

func (outcomeDomain SpotTradeOutcomeDomain) Outcome() dto.SpotTradeOutcomeDto {
	ledger := outcomeDomain.ledger
	grossProfit := ledger.GrossProfit(vo.PositionDirectionLong)
	totalFee := ledger.TotalFee()
	netProfit := grossProfit.Sub(totalFee)
	buyCost := ledger.EntryValue()
	plannedRisk := NewPlannedRiskDomain(
		ledger.AverageEntryPrice(), ledger.EnteredQuantity(), outcomeDomain.record.PlannedStopLossPrice)

	outcomeDto := dto.SpotTradeOutcomeDto{
		GrossProfit: grossProfit,
		TotalFee:    totalFee,
		NetProfit:   netProfit,
		BuyCost:     buyCost,
		PlannedRisk: plannedRisk.Value(),
		RMultiple:   plannedRisk.RMultipleOf(netProfit),
		Excursion: NewTradeExcursionDomain(ledger, vo.PositionDirectionLong, plannedRisk).
			ExcursionFor(outcomeDomain.facts.ExtremesRequested, outcomeDomain.facts.PriceExtremes),
		FloatingProfit:          outcomeDomain.floatingProfit(),
		EntrySlippagePercentage: outcomeDomain.entrySlippagePercentage(),
	}

	if buyCost.IsPositive() {
		returnRate := netProfit.Div(buyCost).InexactFloat64()
		outcomeDto.ReturnRate = &returnRate
	}

	if !plannedRisk.Value().Valid {
		outcomeDto.RMultipleUnavailableReason = rMultipleUnavailableNoStopLoss
	}

	// Only a finished trade has a share captured; while held, the realised part is not the trade's result.
	isHeld := outcomeDomain.record.Status == string(vo.SpotTradeStatusOpen)
	if !isHeld && outcomeDto.Excursion.Available && outcomeDto.Excursion.FavorableProfit.IsPositive() {
		profitCaptureRate := grossProfit.Div(outcomeDto.Excursion.FavorableProfit).InexactFloat64()
		outcomeDto.ProfitCaptureRate = &profitCaptureRate
	}

	return outcomeDto
}

func (outcomeDomain SpotTradeOutcomeDomain) floatingProfit() dto.TradeFloatingDto {
	if outcomeDomain.record.Status != string(vo.SpotTradeStatusOpen) {
		return dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNotOpen}
	}
	if !outcomeDomain.facts.HasLatestPrice {
		return dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNoLatestPrice}
	}

	return dto.TradeFloatingDto{
		Available: true,
		Price:     outcomeDomain.facts.LatestPrice,
		Amount:    outcomeDomain.ledger.OpenProfitAt(vo.PositionDirectionLong, outcomeDomain.facts.LatestPrice),
	}
}

// entrySlippagePercentage is positive when the buy was dearer than the bot's reference price.
func (outcomeDomain SpotTradeOutcomeDomain) entrySlippagePercentage() *float64 {
	referencePrice := outcomeDomain.record.SourceReferencePrice
	if !referencePrice.Valid || !referencePrice.Decimal.IsPositive() {
		return nil
	}

	slippagePercentage := outcomeDomain.ledger.AverageEntryPrice().Sub(referencePrice.Decimal).
		Div(referencePrice.Decimal).Mul(oneHundredPercent).InexactFloat64()

	return &slippagePercentage
}
