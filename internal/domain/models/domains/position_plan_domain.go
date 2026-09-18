package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// noLeverage is a position worth exactly what was put down for it. It is also what
// nothing at all is read as, so "no leverage" and "one times leverage" are the same
// thing everywhere below rather than two cases to keep in step.
var noLeverage = decimal.NewFromInt(1)

// wholePercentage is a distance of the entire price, written as the number somebody
// types. A stop that far away sits exactly at zero, which is absurd but arithmetic;
// past it the price would go negative, which is not.
var wholePercentage = decimal.NewFromInt(100)

// PositionPlanDomain is what a bot suggests putting down, and where it suggests
// getting out — plus every rule about the five figures that decide it.
//
// It exists because the four multiplications behind those numbers are the same four
// every round, against the same unchanged settings, with only the price moving. Its
// owner was doing them on a phone while doing something else, and the round they got
// wrong is the one that costs money.
//
// Its zero value is a bot with no position plan at all: PlanFor answers "no
// suggestion" and nothing downstream has to know that is what it means. That is not a
// failure state — leaving the settings empty is a perfectly ordinary thing to do, and
// the message such a bot sends is word for word the one it sent before plans existed.
type PositionPlanDomain struct {
	// capital being zero is what makes this the zero value, so there is no separate
	// flag that could contradict the figures. Without money there is nothing to
	// stake, and that fact cannot disagree with itself.
	capital    decimal.Decimal
	sizing     PositionSizingDomain
	leverage   decimal.Decimal
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
}

// NewPositionPlanDomain reads the five settings and settles every rule about them.
//
// No capital is the zero value rather than an error: not filling something in is not
// the same as filling it in wrongly, and a bot without a position plan is one this
// system has supported from the day bots existed.
//
// How much one opening stakes is asked of the model a replay already asks, in the same
// words — the three spellings, the default of staking everything, and both refusals
// come from there rather than from a second list that would eventually disagree with
// it.
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

	leverage := settings.Leverage
	if !leverage.IsPositive() {
		leverage = noLeverage
	}

	// Below one is refused rather than read as none. Somebody who typed 0.5 meant
	// something by it — half a position, probably — and quietly reading that as a
	// whole one would double what they asked for without telling them.
	if leverage.LessThan(noLeverage) {
		return PositionPlanDomain{}, fmt.Errorf("槓桿倍數不得小於 1 倍")
	}

	if stopLossError := validatedDistance(settings.StopLossPercentage, "停損距離"); stopLossError != nil {
		return PositionPlanDomain{}, stopLossError
	}

	if takeProfitError := validatedDistance(
		settings.TakeProfitPercentage, "停利距離"); takeProfitError != nil {
		return PositionPlanDomain{}, takeProfitError
	}

	return PositionPlanDomain{
		capital:    settings.Capital,
		sizing:     sizing,
		leverage:   leverage,
		stopLoss:   settings.StopLossPercentage,
		takeProfit: settings.TakeProfitPercentage,
	}, nil
}

// validatedDistance is one exit's distance from the price, checked.
//
// It is shared by both exits because both refusals are about the same two things: a
// negative distance would put a stop on the wrong side of the price, and one past a
// hundred percent would put it below zero. Two copies of that would eventually let
// one exit through a check the other refuses.
func validatedDistance(distance decimal.Decimal, name string) error {
	if distance.IsNegative() {
		return fmt.Errorf("%s不得為負", name)
	}

	if distance.GreaterThan(wholePercentage) {
		return fmt.Errorf("%s不得超過 100%%——那會讓價格變成負數", name)
	}

	return nil
}

// ToSettingsDto is these settings as they are stored and handed back.
func (positionPlanDomain PositionPlanDomain) ToSettingsDto() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              positionPlanDomain.capital,
		SizingMode:           string(positionPlanDomain.sizing.Mode()),
		SizingValue:          positionPlanDomain.sizing.Value(),
		Leverage:             positionPlanDomain.leverage,
		StopLossPercentage:   positionPlanDomain.stopLoss,
		TakeProfitPercentage: positionPlanDomain.takeProfit,
	}
}

// PlanFor is what this round suggests, and whether it suggests anything at all.
//
// One question, four ways to answer it with nothing: no capital was ever set, the
// round is asking to stand aside, the round is asking for no change, or there is no
// price to measure from. They are deliberately one answer — to somebody reading the
// message, "this round has nothing to put down" is a single fact, and four separate
// sentences about it would grow four ways of writing the same paragraph.
//
// The target position is what decides both halves: whether to suggest at all, and
// which way. It comes from the trading mode, which already knows that a spot sell
// clears out and a long-short sell opens the other way — so this reads that answer
// rather than restating it.
func (positionPlanDomain PositionPlanDomain) PlanFor(
	target vo.TargetPositionVo, referencePrice decimal.Decimal, hasReference bool,
) (dto.PositionPlanDto, bool) {
	if !positionPlanDomain.capital.IsPositive() || !hasReference {
		return dto.PositionPlanDto{}, false
	}

	if target != vo.TargetPositionLong && target != vo.TargetPositionShort {
		return dto.PositionPlanDto{}, false
	}

	stake, affordable := positionPlanDomain.sizing.StakeFor(positionPlanDomain.capital)
	if !affordable {
		// Said rather than hidden, and not an error: a fixed amount the capital
		// cannot cover is the same ordinary situation a replay skips an opening for.
		// Printing a stake nobody can put down would be worse than printing none.
		return dto.PositionPlanDto{Stake: stake, Affordable: false}, true
	}

	suggestsShort := target == vo.TargetPositionShort
	notional := stake.Mul(positionPlanDomain.leverage)

	positionPlanDto := dto.PositionPlanDto{
		Stake:         stake,
		Affordable:    true,
		Notional:      notional,
		Leveraged:     positionPlanDomain.leverage.GreaterThan(noLeverage),
		HasStopLoss:   positionPlanDomain.stopLoss.IsPositive(),
		HasTakeProfit: positionPlanDomain.takeProfit.IsPositive(),
		SuggestsShort: suggestsShort,
	}

	if positionPlanDto.HasStopLoss {
		// A stop is the price moving against the position, so it sits below a long
		// and above a short. Getting this backwards is the one mistake here that
		// cannot be seen: the wrong figure is still a plausible price.
		positionPlanDto.StopLossPrice = movedAgainst(
			referencePrice, positionPlanDomain.stopLoss, suggestsShort)
		positionPlanDto.LossAtStop = portionOf(notional, positionPlanDomain.stopLoss)
	}

	if positionPlanDto.HasTakeProfit {
		positionPlanDto.TakeProfitPrice = movedAgainst(
			referencePrice, positionPlanDomain.takeProfit, !suggestsShort)
		positionPlanDto.GainAtTarget = portionOf(notional, positionPlanDomain.takeProfit)
	}

	return positionPlanDto, true
}

// movedAgainst is the price that far away, on the side the caller asked for: upwards
// when raising, downwards when not.
//
// Both exits are the same arithmetic in opposite directions, and both directions flip
// with the position's own — so writing it once is what stops a long's stop and a
// short's take-profit from drifting into two different formulas.
func movedAgainst(
	price decimal.Decimal, distance decimal.Decimal, raising bool,
) decimal.Decimal {
	moved := portionOf(price, distance)
	if raising {
		return price.Add(moved)
	}

	return price.Sub(moved)
}

// portionOf is that percentage of an amount.
func portionOf(amount decimal.Decimal, percentage decimal.Decimal) decimal.Decimal {
	return amount.Mul(percentage).Div(wholePercentage)
}
