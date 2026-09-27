package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const fundingUnavailableNoSettlementData = "noSettlementData"

// ContractTradeFundingDomain charges each settlement on the position held at that moment: a positive rate makes longs pay and shorts receive.
type ContractTradeFundingDomain struct {
	ledger    TradeLedgerDomain
	direction vo.PositionDirectionVo
}

func NewContractTradeFundingDomain(
	ledger TradeLedgerDomain, direction vo.PositionDirectionVo,
) ContractTradeFundingDomain {
	return ContractTradeFundingDomain{ledger: ledger, direction: direction}
}

// FundingFor reports no data when a settlement was due inside the holding window yet none is stored, rather than a misleading zero.
func (fundingDomain ContractTradeFundingDomain) FundingFor(
	facts vo.ContractTradeMarketFactsVo,
) dto.ContractTradeFundingDto {
	if len(facts.FundingSettlements) == 0 {
		if facts.FundingSettlementDue {
			return dto.ContractTradeFundingDto{UnavailableReason: fundingUnavailableNoSettlementData}
		}

		return dto.ContractTradeFundingDto{Available: true, Amount: decimal.Zero}
	}

	received := decimal.Zero
	settlementCount := 0
	for _, settlement := range facts.FundingSettlements {
		position := fundingDomain.ledger.PositionAt(settlement.SettlementTime)
		if !position.IsPositive() {
			continue
		}

		// A missing mark only affects the venue's earliest settlements, long before any journal trade; the entry average stands in.
		markPrice := fundingDomain.ledger.AverageEntryPrice()
		if settlement.MarkPrice.Valid {
			markPrice = settlement.MarkPrice.Decimal
		}

		payment := position.Mul(markPrice).Mul(settlement.FundingRate)
		if fundingDomain.direction == vo.PositionDirectionShort {
			received = received.Add(payment)
		} else {
			received = received.Sub(payment)
		}
		settlementCount++
	}

	return dto.ContractTradeFundingDto{Available: true, Amount: received, SettlementCount: settlementCount}
}
