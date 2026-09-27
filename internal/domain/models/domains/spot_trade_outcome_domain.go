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
	isHeld := outcomeDomain.record.Status == string(vo.SpotTradeStatusOpen)

	floatingProfit := dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNotOpen}
	if isHeld && !outcomeDomain.facts.HasLatestPrice {
		floatingProfit = dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNoLatestPrice}
	}
	if isHeld && outcomeDomain.facts.HasLatestPrice {
		floatingProfit = dto.TradeFloatingDto{
			Available: true,
			Price:     outcomeDomain.facts.LatestPrice,
			Amount:    ledger.OpenProfitAt(vo.PositionDirectionLong, outcomeDomain.facts.LatestPrice),
		}
	}

	// Slippage is positive when the buy was dearer than the bot's reference price.
	entrySlippagePercentage := (*float64)(nil)
	referencePrice := outcomeDomain.record.SourceReferencePrice
	if referencePrice.Valid && referencePrice.Decimal.IsPositive() {
		slippagePercentage := ledger.AverageEntryPrice().Sub(referencePrice.Decimal).
			Div(referencePrice.Decimal).Mul(oneHundredPercent).InexactFloat64()
		entrySlippagePercentage = &slippagePercentage
	}

	outcomeDto := dto.SpotTradeOutcomeDto{
		GrossProfit: grossProfit,
		TotalFee:    totalFee,
		NetProfit:   netProfit,
		BuyCost:     buyCost,
		PlannedRisk: plannedRisk.Value(),
		RMultiple:   plannedRisk.RMultipleOf(netProfit),
		Excursion: NewTradeExcursionDomain(ledger, vo.PositionDirectionLong, plannedRisk).
			ExcursionFor(outcomeDomain.facts.ExtremesRequested, outcomeDomain.facts.PriceExtremes),
		FloatingProfit:          floatingProfit,
		EntrySlippagePercentage: entrySlippagePercentage,
	}

	if buyCost.IsPositive() {
		returnRate := netProfit.Div(buyCost).InexactFloat64()
		outcomeDto.ReturnRate = &returnRate
	}

	if !plannedRisk.Value().Valid {
		outcomeDto.RMultipleUnavailableReason = rMultipleUnavailableNoStopLoss
	}

	// Only a finished trade has a share captured; while held, the realised part is not the trade's result.
	if !isHeld && outcomeDto.Excursion.Available && outcomeDto.Excursion.FavorableProfit.IsPositive() {
		profitCaptureRate := grossProfit.Div(outcomeDto.Excursion.FavorableProfit).InexactFloat64()
		outcomeDto.ProfitCaptureRate = &profitCaptureRate
	}

	return outcomeDto
}
