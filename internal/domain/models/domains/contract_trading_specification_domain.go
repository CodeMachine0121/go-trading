package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// defaultFundingIntervalHours is how often a contract settles its funding rate when
// the venue does not list it among those with a setting of their own. The venue lists
// only the exceptions.
const defaultFundingIntervalHours = 8

// ContractTradingSpecificationDomain is one contract's trading specification, checked
// and with the venue's silences filled in. An instance only exists when every rule
// passed.
type ContractTradingSpecificationDomain struct {
	specification        vo.ContractTradingSpecificationVo
	fundingIntervalHours int
}

// NewContractTradingSpecificationDomain checks one reported specification.
//
// The sizes have to be positive: a tick of zero is a price that cannot move, and a
// minimum of zero is an order of nothing. The rates only have to be non-negative — a
// venue waiving a fee is a venue saying something, not a broken figure.
func NewContractTradingSpecificationDomain(
	specification vo.ContractTradingSpecificationVo,
) (ContractTradingSpecificationDomain, error) {
	positiveFigures := []struct {
		name   string
		figure decimal.Decimal
	}{
		{"價格跳動單位", specification.TickSize},
		{"數量步進", specification.QuantityStep},
		{"最小下單量", specification.MinimumQuantity},
	}
	for _, positiveFigure := range positiveFigures {
		if !positiveFigure.figure.IsPositive() {
			return ContractTradingSpecificationDomain{}, fmt.Errorf(
				"%w: %s必須大於零", ErrContractTradingSpecificationValidation, positiveFigure.name)
		}
	}

	nonNegativeFigures := []struct {
		name   string
		figure decimal.Decimal
	}{
		{"最小名目", specification.MinimumNotional},
		{"維持保證金率", specification.MaintenanceMarginRate},
		{"強平手續費率", specification.LiquidationFeeRate},
	}
	for _, nonNegativeFigure := range nonNegativeFigures {
		if nonNegativeFigure.figure.IsNegative() {
			return ContractTradingSpecificationDomain{}, fmt.Errorf(
				"%w: %s不得為負", ErrContractTradingSpecificationValidation, nonNegativeFigure.name)
		}
	}

	fundingIntervalHours := defaultFundingIntervalHours
	if specification.FundingIntervalHours != nil {
		fundingIntervalHours = *specification.FundingIntervalHours
	}
	if fundingIntervalHours <= 0 {
		return ContractTradingSpecificationDomain{}, fmt.Errorf(
			"%w: 資金費率結算間隔必須大於零", ErrContractTradingSpecificationValidation)
	}

	return ContractTradingSpecificationDomain{
		specification:        specification,
		fundingIntervalHours: fundingIntervalHours,
	}, nil
}

// ApplyTo writes this specification onto a contract, stamped with when it was
// confirmed, and hands the contract back. Nothing else about the contract changes.
func (specificationDomain ContractTradingSpecificationDomain) ApplyTo(
	contractTradingSymbol entities.ContractTradingSymbol, confirmedAt time.Time,
) entities.ContractTradingSymbol {
	fundingIntervalHours := specificationDomain.fundingIntervalHours
	confirmedAtUtc := confirmedAt.UTC()

	contractTradingSymbol.TickSize = decimal.NewNullDecimal(specificationDomain.specification.TickSize)
	contractTradingSymbol.QuantityStep = decimal.NewNullDecimal(specificationDomain.specification.QuantityStep)
	contractTradingSymbol.MinimumQuantity = decimal.NewNullDecimal(specificationDomain.specification.MinimumQuantity)
	contractTradingSymbol.MinimumNotional = decimal.NewNullDecimal(specificationDomain.specification.MinimumNotional)
	contractTradingSymbol.MaintenanceMarginRate = decimal.NewNullDecimal(
		specificationDomain.specification.MaintenanceMarginRate)
	contractTradingSymbol.LiquidationFeeRate = decimal.NewNullDecimal(
		specificationDomain.specification.LiquidationFeeRate)
	contractTradingSymbol.FundingIntervalHours = &fundingIntervalHours
	contractTradingSymbol.SpecificationUpdatedAt = &confirmedAtUtc

	return contractTradingSymbol
}
