package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	excursionUnavailableNoMarketData = "noMarketData"
	excursionUnavailableNotComputed  = "notComputed"
)

// ContractTradeExcursionDomain measures how far the market ran against and for the trade while it was held, on the whole entered quantity.
type ContractTradeExcursionDomain struct {
	ledger      ContractTradeLedgerDomain
	direction   vo.PositionDirectionVo
	plannedRisk PlannedRiskDomain
}

func NewContractTradeExcursionDomain(
	ledger ContractTradeLedgerDomain, direction vo.PositionDirectionVo, plannedRisk PlannedRiskDomain,
) ContractTradeExcursionDomain {
	return ContractTradeExcursionDomain{ledger: ledger, direction: direction, plannedRisk: plannedRisk}
}

func (excursionDomain ContractTradeExcursionDomain) ExcursionFor(
	facts vo.ContractTradeMarketFactsVo,
) dto.ContractTradeExcursionDto {
	if !facts.ExtremesRequested {
		return dto.ContractTradeExcursionDto{UnavailableReason: excursionUnavailableNotComputed}
	}
	if !facts.PriceExtremes.Has {
		return dto.ContractTradeExcursionDto{UnavailableReason: excursionUnavailableNoMarketData}
	}

	adversePrice := facts.PriceExtremes.LowestPrice
	favorablePrice := facts.PriceExtremes.HighestPrice
	if excursionDomain.direction == vo.PositionDirectionShort {
		adversePrice, favorablePrice = favorablePrice, adversePrice
	}

	adverseProfit := excursionDomain.ledger.ProfitAt(excursionDomain.direction, adversePrice)
	favorableProfit := excursionDomain.ledger.ProfitAt(excursionDomain.direction, favorablePrice)

	return dto.ContractTradeExcursionDto{
		Available:          true,
		AdversePrice:       adversePrice,
		FavorablePrice:     favorablePrice,
		AdverseProfit:      adverseProfit,
		FavorableProfit:    favorableProfit,
		AdverseRMultiple:   excursionDomain.plannedRisk.RMultipleOf(adverseProfit),
		FavorableRMultiple: excursionDomain.plannedRisk.RMultipleOf(favorableProfit),
	}
}
