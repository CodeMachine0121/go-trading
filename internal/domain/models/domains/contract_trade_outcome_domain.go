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
	outcomeUnavailableNotClosed              = "notClosed"
)

// ContractTradeOutcomeDomain puts a trade's fills, plan and market facts together into what it came to.
type ContractTradeOutcomeDomain struct {
	record          entities.ContractTradeRecord
	ledger          TradeLedgerDomain
	feeRateMissing  bool
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
		record:         recordDomain.record,
		ledger:         recordDomain.Ledger(),
		feeRateMissing: recordDomain.FeeRateMissing(),
		direction:      recordDomain.Direction(),
		facts:          facts,
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
	isHeld := outcomeDomain.record.Status == string(vo.ContractTradeStatusOpen)

	floatingProfit := dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNotOpen}
	if isHeld && !outcomeDomain.facts.HasLatestPrice {
		floatingProfit = dto.TradeFloatingDto{UnavailableReason: outcomeUnavailableNoLatestPrice}
	}
	if isHeld && outcomeDomain.facts.HasLatestPrice {
		floatingProfit = dto.TradeFloatingDto{
			Available: true,
			Price:     outcomeDomain.facts.LatestPrice,
			Amount:    ledger.OpenProfitAt(outcomeDomain.direction, outcomeDomain.facts.LatestPrice),
		}
	}

	liquidation := dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNotOpen}
	switch {
	case !isHeld:
	case !outcomeDomain.rulesRequested:
		liquidation = dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNotComputed}
	case !outcomeDomain.hasTradingRules:
		liquidation = dto.ContractTradeLiquidationDto{UnavailableReason: outcomeUnavailableNoTradingSpecification}
	default:
		// The replay's isolated-margin formula on what is still held, so the journal and the replay never disagree.
		averageEntryPrice := ledger.AverageEntryPrice()
		position := ledger.Position()
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
		liquidation = dto.ContractTradeLiquidationDto{Available: true, CannotBeLiquidated: true}
		if liquidationPrice.IsPositive() {
			liquidation = dto.ContractTradeLiquidationDto{
				Available: true,
				Price:     outcomeDomain.tradingRules.RoundedToTick(liquidationPrice),
			}
		}
	}

	// Slippage is positive when the fill was worse than the bot's reference price: paying more on a long, receiving less on a short.
	entrySlippagePercentage := (*float64)(nil)
	referencePrice := outcomeDomain.record.SourceReferencePrice
	if referencePrice.Valid && referencePrice.Decimal.IsPositive() {
		difference := ledger.AverageEntryPrice().Sub(referencePrice.Decimal)
		if outcomeDomain.direction == vo.PositionDirectionShort {
			difference = difference.Neg()
		}
		slippagePercentage := difference.Div(referencePrice.Decimal).Mul(oneHundredPercent).InexactFloat64()
		entrySlippagePercentage = &slippagePercentage
	}

	entryNotional := ledger.EntryValue()
	entryMargin := entryNotional.Div(outcomeDomain.record.Leverage)

	outcomeDto := dto.ContractTradeOutcomeDto{
		GrossProfit:              grossProfit,
		TotalFee:                 totalFee,
		FeeRateMissing:           outcomeDomain.feeRateMissing,
		Funding:                  funding,
		NetProfit:                netProfit,
		NetProfitExcludesFunding: !funding.Available,
		PlannedRisk:              plannedRisk.Value(),
		RMultiple:                plannedRisk.RMultipleOf(netProfit),
		Excursion: NewTradeExcursionDomain(ledger, outcomeDomain.direction, plannedRisk).
			ExcursionFor(outcomeDomain.facts.ExtremesRequested, outcomeDomain.facts.PriceExtremes),
		FloatingProfit:          floatingProfit,
		LiquidationPrice:        liquidation,
		EntrySlippagePercentage: entrySlippagePercentage,
		EntryNotional:           entryNotional,
		EntryMargin:             entryMargin,
		ImplausibleFeeFillIDs:   ledger.ImplausibleFeeFillIDs(),
	}

	switch {
	case isHeld:
		outcomeDto.ReturnOnMarginUnavailableReason = outcomeUnavailableNotClosed
	case entryMargin.IsPositive():
		returnOnMarginPercentage := netProfit.Div(entryMargin).Mul(oneHundredPercent).InexactFloat64()
		outcomeDto.ReturnOnMarginPercentage = &returnOnMarginPercentage
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
