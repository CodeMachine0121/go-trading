package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

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
	leverage   LeverageMultiplierDomain
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

	// Asked of the model a replay asks, so that the same figure typed into either
	// comes back with the same sentence.
	leverage, leverageError := NewLeverageMultiplierDomain(settings.Leverage)
	if leverageError != nil {
		return PositionPlanDomain{}, leverageError
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
//
// A replay's own exit distances are checked here too, in these same words. The two
// are different things — one says what a bot should suggest each round, the other how
// one replay simulates — but the same 150 typed into either has to come back with the
// same sentence, and only one copy of a sentence can stay true to itself.
func validatedDistance(distance decimal.Decimal, name string) error {
	if distance.IsNegative() {
		return fmt.Errorf("%s不得為負", name)
	}

	if distance.GreaterThan(oneHundredPercent) {
		return fmt.Errorf("%s不得超過 100%%——那會讓價格變成負數", name)
	}

	return nil
}

// IsBorrowed is whether what this plan suggests putting on is worth more than the
// money behind it.
//
// It is asked of the multiplier itself, the same model a replay asks, so that a bot
// and a replay cannot end up disagreeing about when a figure counts as a loan.
func (positionPlanDomain PositionPlanDomain) IsBorrowed() bool {
	return positionPlanDomain.leverage.IsBorrowed()
}

// ToSettingsDto is these settings as they are stored and handed back.
//
// A bot with no position plan hands back nothing at all, leverage included. The
// multiplier answers one for a plan that borrows nothing, which is right everywhere
// it is used as arithmetic — but written down it would put a multiplier on a bot that
// has no stake to multiply, and hand that back on every read.
func (positionPlanDomain PositionPlanDomain) ToSettingsDto() dto.PositionPlanSettingsDto {
	if !positionPlanDomain.capital.IsPositive() {
		return dto.PositionPlanSettingsDto{}
	}

	return dto.PositionPlanSettingsDto{
		Capital:              positionPlanDomain.capital,
		SizingMode:           string(positionPlanDomain.sizing.Mode()),
		SizingValue:          positionPlanDomain.sizing.Value(),
		Leverage:             positionPlanDomain.leverage.Multiplier(),
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

	// No costs and no borrowing, both stated rather than implied. What a bot suggests
	// each round is advice about a trade nobody has placed, so there is no charge to
	// have been paid — and this model applies its own leverage to the notional below,
	// which is a suggestion about what to open rather than a loan a replay is carrying.
	stake, affordable := positionPlanDomain.sizing.StakeFor(
		positionPlanDomain.capital, BacktestTransactionCostsDomain{}, BacktestLeverageDomain{})
	if !affordable {
		// Said rather than hidden, and not an error: a fixed amount the capital
		// cannot cover is the same ordinary situation a replay skips an opening for.
		// Printing a stake nobody can put down would be worse than printing none.
		return dto.PositionPlanDto{Stake: stake, Affordable: false}, true
	}

	suggestsShort := target == vo.TargetPositionShort
	notional := stake.Mul(positionPlanDomain.leverage.Multiplier())

	positionPlanDto := dto.PositionPlanDto{
		Stake:         stake,
		Affordable:    true,
		Notional:      notional,
		Leveraged:     positionPlanDomain.leverage.IsBorrowed(),
		HasStopLoss:   positionPlanDomain.stopLoss.IsPositive(),
		HasTakeProfit: positionPlanDomain.takeProfit.IsPositive(),
		SuggestsShort: suggestsShort,
	}

	if positionPlanDto.HasStopLoss {
		// A stop is the price moving against the position, so it sits below a long
		// and above a short. Getting this backwards is the one mistake here that
		// cannot be seen: the wrong figure is still a plausible price.
		// A short's stop is above the price; a long's is below.
		positionPlanDto.StopLossPrice = movedBy(
			referencePrice, positionPlanDomain.stopLoss, suggestsShort)
		positionPlanDto.LossAtStop = portionOf(notional, positionPlanDomain.stopLoss)
	}

	if positionPlanDto.HasTakeProfit {
		// And the target is on the other side of the price from the stop, always.
		positionPlanDto.TakeProfitPrice = movedBy(
			referencePrice, positionPlanDomain.takeProfit, !suggestsShort)
		positionPlanDto.GainAtTarget = portionOf(notional, positionPlanDomain.takeProfit)
	}

	return positionPlanDto, true
}

// movedBy is the price that far away, on the side the caller asked for.
//
// Named after the arithmetic rather than after either exit, because it serves both and
// they lie on opposite sides: a stop is the price moving against the position, a
// target is it moving in favour. Calling it "moved against" would be right for one
// caller and a lie to the other.
//
// Both directions also flip with the position's own, so writing it once is what stops
// a long's stop and a short's target from drifting into two different formulas.
func movedBy(
	price decimal.Decimal, distance decimal.Decimal, upwards bool,
) decimal.Decimal {
	moved := portionOf(price, distance)
	if upwards {
		return price.Add(moved)
	}

	return price.Sub(moved)
}

// portionOf is that percentage of an amount.
func portionOf(amount decimal.Decimal, percentage decimal.Decimal) decimal.Decimal {
	return amount.Mul(percentage).Div(oneHundredPercent)
}
