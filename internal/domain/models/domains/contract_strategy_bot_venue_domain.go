package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractStrategyBotVenueDomain is what trading one perpetual contract looks like at
// the moment a contract bot suggests a position on it: the venue's trading rules — how
// prices and quantities step, the smallest order, the maintenance margin ladder — and
// the funding rate most recently settled.
//
// Either half may be missing, and neither is a failure. A contract whose specification
// has not been recorded yet still gets a suggestion, only not one rounded to the venue
// or with a liquidation price; a contract with no settlement yet gets no funding
// estimate. Saying so is the suggestion's job, and it is far more use to the reader
// than a bot that goes quiet until the next refresh.
//
// The rules are the contract replay's own model, so a suggestion and a replay of the
// same figures can never disagree about what the venue would have taken.
type ContractStrategyBotVenueDomain struct {
	tradingRules    ContractTradingRulesDomain
	hasTradingRules bool
	fundingRate     decimal.Decimal
	hasFundingRate  bool
	// fundingIntervalHours is how often funding settles, zero when the specification
	// does not say.
	fundingIntervalHours int
}

// NewContractStrategyBotVenueDomain reads the contract as it is stored: its entry and
// whether there was one, its ladder, and its latest funding settlement and whether
// there was one.
func NewContractStrategyBotVenueDomain(
	contractTradingSymbol entities.ContractTradingSymbol,
	isRegistered bool,
	maintenanceMarginTiers []entities.ContractMaintenanceMarginTier,
	latestSettlement entities.ContractFundingRateSettlement,
	hasSettlement bool,
) ContractStrategyBotVenueDomain {
	// The replay refuses a contract without a specification, and for a replay that is
	// right. For a suggestion it only means the venue's rules cannot be applied yet —
	// which is what hasTradingRules says, rather than passing the replay's sentence on.
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

// TradingRules is the venue's rules for this contract, and whether they are known.
func (venueDomain ContractStrategyBotVenueDomain) TradingRules() (ContractTradingRulesDomain, bool) {
	return venueDomain.tradingRules, venueDomain.hasTradingRules
}

// WithFundingEstimate is this suggestion with what one funding settlement at the rate
// last settled here would come to on its notional: a positive rate is paid by a long and
// received by a short, and a negative one the other way round. With no settlement yet it
// only says how often funding settles.
//
// It is the venue's to say, because every figure it reads but the notional and the
// direction is the venue's own.
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
