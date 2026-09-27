package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	excursionUnavailableNoMarketData = "noMarketData"
	excursionUnavailableNotComputed  = "notComputed"
)

// TradeExcursionDomain measures how far the market ran against and for the trade while it was held, on the whole entered quantity.
type TradeExcursionDomain struct {
	ledger      TradeLedgerDomain
	direction   vo.PositionDirectionVo
	plannedRisk PlannedRiskDomain
}

func NewTradeExcursionDomain(
	ledger TradeLedgerDomain, direction vo.PositionDirectionVo, plannedRisk PlannedRiskDomain,
) TradeExcursionDomain {
	return TradeExcursionDomain{ledger: ledger, direction: direction, plannedRisk: plannedRisk}
}

// ExcursionFor is not computed where only a summary was asked for, and unavailable when the market left no candles for the holding.
func (excursionDomain TradeExcursionDomain) ExcursionFor(
	extremesRequested bool, priceExtremes vo.PriceExtremesVo,
) dto.TradeExcursionDto {
	if !extremesRequested {
		return dto.TradeExcursionDto{UnavailableReason: excursionUnavailableNotComputed}
	}
	if !priceExtremes.Has {
		return dto.TradeExcursionDto{UnavailableReason: excursionUnavailableNoMarketData}
	}

	adversePrice := priceExtremes.LowestPrice
	favorablePrice := priceExtremes.HighestPrice
	if excursionDomain.direction == vo.PositionDirectionShort {
		adversePrice, favorablePrice = favorablePrice, adversePrice
	}

	adverseProfit := excursionDomain.ledger.ProfitAt(excursionDomain.direction, adversePrice)
	favorableProfit := excursionDomain.ledger.ProfitAt(excursionDomain.direction, favorablePrice)

	return dto.TradeExcursionDto{
		Available:          true,
		AdversePrice:       adversePrice,
		FavorablePrice:     favorablePrice,
		AdverseProfit:      adverseProfit,
		FavorableProfit:    favorableProfit,
		AdverseRMultiple:   excursionDomain.plannedRisk.RMultipleOf(adverseProfit),
		FavorableRMultiple: excursionDomain.plannedRisk.RMultipleOf(favorableProfit),
	}
}
