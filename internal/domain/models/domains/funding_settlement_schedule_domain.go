package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// FundingSettlementScheduleDomain knows when a contract settles funding: every interval hours on the UTC clock.
type FundingSettlementScheduleDomain struct {
	interval time.Duration
}

func NewFundingSettlementScheduleDomain(contractTradingSymbol entities.ContractTradingSymbol) FundingSettlementScheduleDomain {
	intervalHours := defaultFundingIntervalHours
	if contractTradingSymbol.FundingIntervalHours != nil && *contractTradingSymbol.FundingIntervalHours > 0 {
		intervalHours = *contractTradingSymbol.FundingIntervalHours
	}

	return FundingSettlementScheduleDomain{interval: time.Duration(intervalHours) * time.Hour}
}

// IsDueBetween tells whether a settlement time falls after start and no later than end.
func (scheduleDomain FundingSettlementScheduleDomain) IsDueBetween(start time.Time, end time.Time) bool {
	nextSettlement := start.UTC().Truncate(scheduleDomain.interval).Add(scheduleDomain.interval)

	return !nextSettlement.After(end.UTC())
}
