package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractAutoOrderIntentDomain is what a contract round would do with real money, read off the same position plan its message shows, so the order never disagrees with what the owner was told.
type ContractAutoOrderIntentDomain struct {
	round dto.StrategyBotRoundDto
}

func NewContractAutoOrderIntentDomain(round dto.StrategyBotRoundDto) ContractAutoOrderIntentDomain {
	return ContractAutoOrderIntentDomain{round: round}
}

// ToDto is false for a spot round, and for a round whose target cannot be named (an unreadable trading mode), since neither has anything to send.
func (intentDomain ContractAutoOrderIntentDomain) ToDto() (dto.ContractAutoOrderIntentDto, bool) {
	market := NewStrategyBotMarketDomain(intentDomain.round.MarketDataKind, intentDomain.round.ContractTradingMode)
	if !market.IsContract() {
		return dto.ContractAutoOrderIntentDto{}, false
	}

	target := market.TargetFor(NewSignalDomainOf(vo.SignalVo(intentDomain.round.Verdict)))
	if target != vo.TargetPositionLong && target != vo.TargetPositionShort && target != vo.TargetPositionFlat {
		return dto.ContractAutoOrderIntentDto{}, false
	}

	settings := intentDomain.round.PositionPlanSettings
	leverage := settings.Leverage
	if !leverage.IsPositive() {
		leverage = decimal.NewFromInt(1)
	}

	intent := dto.ContractAutoOrderIntentDto{
		TargetPosition:       string(target),
		Leverage:             leverage,
		StopLossPercentage:   settings.StopLossPercentage,
		TakeProfitPercentage: settings.TakeProfitPercentage,
	}
	if target == vo.TargetPositionFlat {
		return intent, true
	}

	intent.OpenQuantity, intent.HasOpenQuantity, intent.NoOpenReason = intentDomain.openQuantity()

	return intent, true
}

// openQuantity takes the suggested quantity only when the suggestion is one the venue would accept; every other case names why nothing is opened, in the words the round's message uses.
func (intentDomain ContractAutoOrderIntentDomain) openQuantity() (decimal.Decimal, bool, string) {
	positionPlan := intentDomain.round.PositionPlan

	if !intentDomain.round.PositionPlanSettings.Capital.IsPositive() {
		return decimal.Zero, false, "沒有部位規劃，不下單"
	}
	if !intentDomain.round.HasPositionPlan {
		return decimal.Zero, false, "這一輪算不出建議部位，不下單"
	}
	if !positionPlan.Affordable {
		return decimal.Zero, false, fmt.Sprintf("部位資金不足，押不下 %s，不下單", positionPlan.Stake.String())
	}
	if positionPlan.HasVenueRefusal {
		refusal := positionPlan.VenueRefusal

		switch vo.ContractOrderRefusalReasonVo(refusal.Reason) {
		case vo.ContractOrderRefusalBelowMinimumQuantity:
			return decimal.Zero, false, fmt.Sprintf("交易所不收這一筆：數量 %s 低於最小下單量 %s",
				refusal.Quantity.String(), refusal.MinimumQuantity.String())
		case vo.ContractOrderRefusalBelowMinimumNotional:
			return decimal.Zero, false, fmt.Sprintf("交易所不收這一筆：名目 %s 低於最小名目 %s",
				refusal.Notional.String(), refusal.MinimumNotional.String())
		}

		return decimal.Zero, false, fmt.Sprintf("交易所不收這一筆：名目 %s 那一級最高只能開 %d 倍",
			refusal.Notional.String(), refusal.TierMaximumLeverage)
	}
	if !positionPlan.HasQuantity || !positionPlan.Quantity.IsPositive() {
		return decimal.Zero, false, "這個合約標的還沒有交易規格，算不出下單數量，不下單"
	}

	return positionPlan.Quantity, true, ""
}
