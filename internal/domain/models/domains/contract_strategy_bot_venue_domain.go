package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractStrategyBotVenueDomain holds the venue's trading rules and latest funding rate for a bot suggestion; either may be missing, which degrades the suggestion rather than failing it.
type ContractStrategyBotVenueDomain struct {
	tradingRules    ContractTradingRulesDomain
	hasTradingRules bool
	fundingRate     decimal.Decimal
	hasFundingRate  bool
	// fundingIntervalHours is zero when the specification does not say.
	fundingIntervalHours int
}

func NewContractStrategyBotVenueDomain(
	contractTradingSymbol entities.ContractTradingSymbol,
	isRegistered bool,
	maintenanceMarginTiers []entities.ContractMaintenanceMarginTier,
	latestSettlement entities.ContractFundingRateSettlement,
	hasSettlement bool,
) ContractStrategyBotVenueDomain {
	// A missing specification only means the rules cannot be applied yet, reported via hasTradingRules instead of the replay's error.
	tradingRules, rulesError := NewContractTradingRulesDomain(
		contractTradingSymbol, isRegistered, maintenanceMarginTiers)

	fundingIntervalHours := 0
	if contractTradingSymbol.FundingIntervalHours != nil {
		fundingIntervalHours = *contractTradingSymbol.FundingIntervalHours
	}

	return ContractStrategyBotVenueDomain{
		tradingRules:         tradingRules,
		hasTradingRules:      rulesError == nil,
		fundingRate:          latestSettlement.FundingRate,
		hasFundingRate:       hasSettlement,
		fundingIntervalHours: fundingIntervalHours,
	}
}

func (venueDomain ContractStrategyBotVenueDomain) TradingRules() (ContractTradingRulesDomain, bool) {
	return venueDomain.tradingRules, venueDomain.hasTradingRules
}

// WithFundingEstimate adds one settlement's payment at the last rate on the notional (positive rate: long pays, short receives), or only the interval when no settlement exists.
func (venueDomain ContractStrategyBotVenueDomain) WithFundingEstimate(
	positionPlanDto dto.PositionPlanDto,
) dto.PositionPlanDto {
	positionPlanDto.FundingIntervalHours = venueDomain.fundingIntervalHours

	if !venueDomain.hasFundingRate {
		return positionPlanDto
	}

	fundingPayment := positionPlanDto.Notional.Mul(venueDomain.fundingRate)
	if positionPlanDto.Direction == string(vo.PositionDirectionShort) {
		fundingPayment = fundingPayment.Neg()
	}

	positionPlanDto.FundingRate = venueDomain.fundingRate
	positionPlanDto.HasFundingRate = true
	positionPlanDto.FundingPayment = fundingPayment

	return positionPlanDto
}
