package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// PositionPlanDomain is what a bot suggests putting down, and where it suggests
// getting out — plus every rule about the four figures that decide it.
//
// It exists because the multiplications behind those numbers are the same ones every
// round, against the same unchanged settings, with only the price moving. Its
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
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
	// exitLevels places the two exits around a price for whichever way a position
	// faces — the very model a replay places its exits with, so a bot's suggestion and
	// a replay of the same distances can never put a stop in two different places.
	exitLevels BacktestExitLevelsDomain
	// leverage is how many times its margin a contract suggestion carries. Zero is a
	// spot plan: nothing is borrowed, and the notional is the stake itself. Whether a
	// figure is allowed was settled where the bot was saved, not here — see
	// NewPositionPlanDomain.
	leverage decimal.Decimal
}

// NewPositionPlanDomain reads the four settings and settles every rule about them.
//
// **Every rule here is about a figure that is stored**, because this runs again on
// every round of every bot, from settings saved long ago. A rule about something a
// caller merely declared — borrowing, most of all — belongs where the bot is settled:
// refusing it here would stop a bot that predates the rule, silently, every round.
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

// ToSettingsDto is these settings as they are stored and handed back.
//
// A bot with no position plan hands back nothing at all.
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

// PlanFor is what this round suggests, and whether it suggests anything at all.
//
// One question, four ways to answer it with nothing: no capital was ever set, the
// round is asking to stand aside, the round is asking for no change, or there is no
// price to measure from. They are deliberately one answer — to somebody reading the
// message, "this round has nothing to put down" is a single fact, and four separate
// sentences about it would grow four ways of writing the same paragraph.
//
// The target position is what decides whether to suggest anything at all, and which
// way. A spot target only ever faces long; a contract target may face short too, and
// then both exits swap sides. Closing either side is being asked to hold nothing, and
// there is nothing to suggest opening about that.
func (positionPlanDomain PositionPlanDomain) PlanFor(
	target vo.TargetPositionVo, referencePrice decimal.Decimal, hasReference bool,
) (dto.PositionPlanDto, bool) {
	if !positionPlanDomain.capital.IsPositive() || !hasReference {
		return dto.PositionPlanDto{}, false
	}

	if target != vo.TargetPositionLong && target != vo.TargetPositionShort {
		return dto.PositionPlanDto{}, false
	}

	// No costs, stated rather than implied. What a bot suggests each round is advice
	// about a trade nobody has placed, so there is no charge to have been paid — and
	// that line is the only place the decision to leave live advice alone is visible.
	stake, affordable := positionPlanDomain.sizing.StakeFor(
		positionPlanDomain.capital, BacktestTransactionCostsDomain{})
	if !affordable {
		// Said rather than hidden, and not an error: a fixed amount the capital
		// cannot cover is the same ordinary situation a replay skips an opening for.
		// Printing a stake nobody can put down would be worse than printing none.
		return dto.PositionPlanDto{Stake: stake, Affordable: false}, true
	}

	// A spot plan borrows nothing, which is the same as carrying it once.
	leverage := decimal.Max(positionPlanDomain.leverage, oneWhole)
	notional := stake.Mul(leverage)
	direction := vo.PositionDirectionLong
	if target == vo.TargetPositionShort {
		direction = vo.PositionDirectionShort
	}

	// Which side each exit lies on is the direction's, placed by the model a replay
	// uses: a stop is the price moving against the position — below a long one, above
	// a short one — and the target is always on the other side.
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
		// What moves with the price is the notional, not the margin: at five times, a
		// two percent move costs ten percent of what was put down.
		positionPlanDto.LossAtStop = portionOf(notional, positionPlanDomain.stopLoss)
		// Only a borrowed position can be closed out before its stop. The check is the
		// plain one — the move that eats the whole margin — and leaves the maintenance
		// margin out, which the message says out loud.
		positionPlanDto.LiquidatesBeforeStop = positionPlanDomain.leverage.IsPositive() &&
			positionPlanDomain.stopLoss.Mul(leverage).GreaterThanOrEqual(oneHundredPercent)
	}

	if positionPlanDto.HasTakeProfit {
		positionPlanDto.GainAtTarget = portionOf(notional, positionPlanDomain.takeProfit)
	}

	return positionPlanDto, true
}

// portionOf is that percentage of an amount.
//
// It is the one piece of arithmetic every exit shares — how far from a price a
// distance actually is — and which side that lands on is left to whoever is placing
// it, because that depends on which way the position faces.
func portionOf(amount decimal.Decimal, percentage decimal.Decimal) decimal.Decimal {
	return amount.Mul(percentage).Div(oneHundredPercent)
}
