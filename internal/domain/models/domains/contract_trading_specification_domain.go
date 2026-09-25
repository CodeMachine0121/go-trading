package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// defaultFundingIntervalHours applies to contracts the venue does not list, since it lists only the exceptions.
const defaultFundingIntervalHours = 8

// ContractTradingSpecificationDomain is a validated specification with the venue's omissions filled in.
type ContractTradingSpecificationDomain struct {
	specification        vo.ContractTradingSpecificationVo
	fundingIntervalHours int
}

// NewContractTradingSpecificationDomain requires sizes to be positive but rates only non-negative, since a waived fee is legitimate.
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

// ApplyTo stamps this specification onto the contract with its confirmation time, leaving everything else unchanged.
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
