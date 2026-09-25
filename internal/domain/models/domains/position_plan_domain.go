package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// PositionPlanDomain computes a bot's suggested stake and exits each round; its zero value is a bot with no plan, for which PlanFor simply suggests nothing.
type PositionPlanDomain struct {
	// A zero capital is what makes this the zero value, so no separate flag can contradict it.
	capital    decimal.Decimal
	sizing     PositionSizingDomain
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
	// exitLevels is the replay's own model, so a suggestion and a replay can never place a stop differently.
	exitLevels BacktestExitLevelsDomain
	// leverage zero means a spot plan whose notional is the stake; allowed values were settled at save time.
	leverage decimal.Decimal
}

// NewPositionPlanDomain only enforces rules on stored figures, since it reruns every round on old settings; no capital yields the zero value rather than an error.
func NewPositionPlanDomain(
	settings dto.PositionPlanSettingsDto,
) (PositionPlanDomain, error) {
	if !settings.Capital.IsPositive() {
		return PositionPlanDomain{}, nil
	}

	sizing, sizingError := NewPositionSizingDomain(settings.SizingMode, settings.SizingValue)
	if sizingError != nil {
		return PositionPlanDomain{}, sizingError
	}

	exitLevels, exitLevelsError := NewBacktestExitLevelsDomain(
		settings.StopLossPercentage, settings.TakeProfitPercentage)
	if exitLevelsError != nil {
		return PositionPlanDomain{}, exitLevelsError
	}

	return PositionPlanDomain{
		capital:    settings.Capital,
		sizing:     sizing,
		stopLoss:   settings.StopLossPercentage,
		takeProfit: settings.TakeProfitPercentage,
		exitLevels: exitLevels,
		leverage:   settings.Leverage,
	}, nil
}

// validatedDistance is shared by both exits and by replays so the same input always gets the same refusal.
func validatedDistance(distance decimal.Decimal, name string) error {
	if distance.IsNegative() {
		return fmt.Errorf("%s不得為負", name)
	}

	if distance.GreaterThan(oneHundredPercent) {
		return fmt.Errorf("%s不得超過 100%%——那會讓價格變成負數", name)
	}

	return nil
}

// ToSettingsDto hands back nothing for a bot without a plan.
func (positionPlanDomain PositionPlanDomain) ToSettingsDto() dto.PositionPlanSettingsDto {
	if !positionPlanDomain.capital.IsPositive() {
		return dto.PositionPlanSettingsDto{}
	}

	return dto.PositionPlanSettingsDto{
		Capital:              positionPlanDomain.capital,
		SizingMode:           string(positionPlanDomain.sizing.Mode()),
		SizingValue:          positionPlanDomain.sizing.Value(),
		StopLossPercentage:   positionPlanDomain.stopLoss,
		TakeProfitPercentage: positionPlanDomain.takeProfit,
		Leverage:             positionPlanDomain.leverage,
	}
}

// PlanFor suggests nothing when there is no capital, no reference price, or a target that is not long or short; short targets swap both exits.
func (positionPlanDomain PositionPlanDomain) PlanFor(
	target vo.TargetPositionVo, referencePrice decimal.Decimal, hasReference bool,
) (dto.PositionPlanDto, bool) {
	if !positionPlanDomain.capital.IsPositive() || !hasReference {
		return dto.PositionPlanDto{}, false
	}

	if target != vo.TargetPositionLong && target != vo.TargetPositionShort {
		return dto.PositionPlanDto{}, false
	}

	// No transaction costs: live advice is about a trade nobody has placed.
	stake, affordable := positionPlanDomain.sizing.StakeFor(
		positionPlanDomain.capital, BacktestTransactionCostsDomain{})
	if !affordable {
		// Not an error: an unaffordable fixed amount is reported rather than printing a stake nobody can put down.
		return dto.PositionPlanDto{Stake: stake, Affordable: false}, true
	}

	// A spot plan borrows nothing, the same as 1x.
	leverage := decimal.Max(positionPlanDomain.leverage, oneWhole)
	notional := stake.Mul(leverage)
	direction := vo.PositionDirectionLong
	if target == vo.TargetPositionShort {
		direction = vo.PositionDirectionShort
	}

	exitPrices := positionPlanDomain.exitLevels.PricesFacing(direction, referencePrice)

	positionPlanDto := dto.PositionPlanDto{
		Stake:           stake,
		Affordable:      true,
		StopLossPrice:   exitPrices.StopLossPrice,
		HasStopLoss:     exitPrices.HasStopLoss,
		TakeProfitPrice: exitPrices.TakeProfitPrice,
		HasTakeProfit:   exitPrices.HasTakeProfit,
		Direction:       string(direction),
		Leverage:        leverage,
		Notional:        notional,
	}

	if positionPlanDto.HasStopLoss {
		// Losses scale with the notional, not the margin.
		positionPlanDto.LossAtStop = portionOf(notional, positionPlanDomain.stopLoss)
		// A simple whole-margin check that ignores maintenance margin, as the message states.
		positionPlanDto.LiquidatesBeforeStop = positionPlanDomain.leverage.IsPositive() &&
			positionPlanDomain.stopLoss.Mul(leverage).GreaterThanOrEqual(oneHundredPercent)
	}

	if positionPlanDto.HasTakeProfit {
		positionPlanDto.GainAtTarget = portionOf(notional, positionPlanDomain.takeProfit)
	}

	return positionPlanDto, true
}

// NeedsVenue reports whether a suggestion needs the venue's rules at all, so a round with nothing affordable to place reads nothing from the venue.
func (positionPlanDomain PositionPlanDomain) NeedsVenue(target vo.TargetPositionVo, hasReference bool) bool {
	if !positionPlanDomain.capital.IsPositive() || !hasReference ||
		(target != vo.TargetPositionLong && target != vo.TargetPositionShort) {
		return false
	}

	_, affordable := positionPlanDomain.sizing.StakeFor(
		positionPlanDomain.capital, BacktestTransactionCostsDomain{})

	return affordable
}

// PlanOnContractVenue opens the suggestion with the contract replay's own opening model (no costs or slippage), so quantity, margin, ticks, refusals and liquidation match a replay.
// Without known trading rules it falls back to PlanFor and says so; the funding estimate is added either way.
func (positionPlanDomain PositionPlanDomain) PlanOnContractVenue(
	target vo.TargetPositionVo,
	referencePrice decimal.Decimal,
	referenceTime time.Time,
	venue ContractStrategyBotVenueDomain,
) (dto.PositionPlanDto, bool) {
	plainPlan, suggests := positionPlanDomain.PlanFor(target, referencePrice, true)
	if !suggests || !plainPlan.Affordable {
		return plainPlan, suggests
	}

	tradingRules, hasTradingRules := venue.TradingRules()
	if !hasTradingRules {
		plainPlan.ForContract = true
		plainPlan.LacksTradingSpecification = true

		return venue.WithFundingEstimate(plainPlan), true
	}

	leverage := decimal.Max(positionPlanDomain.leverage, oneWhole)
	direction := vo.PositionDirectionVo(plainPlan.Direction)
	position, outcome := NewContractPositionTermsDomain(
		NewBacktestPositionTermsDomain(
			positionPlanDomain.sizing, positionPlanDomain.exitLevels, BacktestTransactionCostsDomain{}),
		leverage, BacktestSlippageDomain{}, tradingRules,
	).OpenFor(direction, referenceTime, referencePrice, positionPlanDomain.capital)

	// An unopenable price keeps the plain suggestion rather than deriving a liquidation from a position never opened.
	if outcome == vo.ContractOpeningUnaffordable {
		plainPlan.ForContract = true

		return venue.WithFundingEstimate(plainPlan), true
	}

	if outcome == vo.ContractOpeningBlockedByTradingRules {
		refusal, _ := tradingRules.RefusalFor(
			tradingRules.QuantityFor(plainPlan.Stake.Mul(leverage), referencePrice), referencePrice, leverage)

		return dto.PositionPlanDto{
			Stake:           plainPlan.Stake,
			Affordable:      true,
			Direction:       plainPlan.Direction,
			Leverage:        leverage,
			Notional:        plainPlan.Notional,
			ForContract:     true,
			VenueRefusal:    refusal.ToDto(),
			HasVenueRefusal: true,
		}, true
	}

	notional := position.Quantity().Mul(position.EntryPrice())
	exitPrices := position.ExitPrices()
	// Judged on the tick-rounded price so a liquidation price of zero is never printed.
	liquidationPrice := position.LiquidationPrice()
	roundedLiquidationPrice := tradingRules.RoundedToTick(liquidationPrice)
	cannotBeLiquidated := !liquidationPrice.IsPositive() || !roundedLiquidationPrice.IsPositive()

	positionPlanDto := dto.PositionPlanDto{
		Stake:                       position.OpeningMargin(),
		Affordable:                  true,
		StopLossPrice:               exitPrices.StopLossPrice,
		HasStopLoss:                 exitPrices.HasStopLoss,
		TakeProfitPrice:             exitPrices.TakeProfitPrice,
		HasTakeProfit:               exitPrices.HasTakeProfit,
		Direction:                   plainPlan.Direction,
		Leverage:                    leverage,
		Notional:                    notional,
		ForContract:                 true,
		Quantity:                    position.Quantity(),
		HasQuantity:                 true,
		CannotBeLiquidated:          cannotBeLiquidated,
		HasLiquidationPrice:         !cannotBeLiquidated,
		LiquidationFromSmallestTier: !tradingRules.HasLadder(),
	}

	if !cannotBeLiquidated {
		positionPlanDto.LiquidationPrice = roundedLiquidationPrice
	}

	if positionPlanDto.HasStopLoss {
		positionPlanDto.LossAtStop = portionOf(notional, positionPlanDomain.stopLoss)
		// Same rule as the replay: compare against the unrounded price, with a stop exactly at it firing first.
		positionPlanDto.LiquidatesBeforeStop = !cannotBeLiquidated &&
			((direction == vo.PositionDirectionLong &&
				positionPlanDto.StopLossPrice.LessThan(liquidationPrice)) ||
				(direction == vo.PositionDirectionShort &&
					positionPlanDto.StopLossPrice.GreaterThan(liquidationPrice)))
	}

	if positionPlanDto.HasTakeProfit {
		positionPlanDto.GainAtTarget = portionOf(notional, positionPlanDomain.takeProfit)
	}

	return venue.WithFundingEstimate(positionPlanDto), true
}

// portionOf is that percentage of an amount; which side of the price it lands on is the caller's concern.
func portionOf(amount decimal.Decimal, percentage decimal.Decimal) decimal.Decimal {
	return amount.Mul(percentage).Div(oneHundredPercent)
}
