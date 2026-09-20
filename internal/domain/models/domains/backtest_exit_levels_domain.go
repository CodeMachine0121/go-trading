package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestExitLevelsDomain is how far from its entry one replay's positions are
// willing to be wrong, and how far right is far enough — the two distances, checked.
//
// It exists because every report card this system had produced until now assumed a bet
// was held until the signal that closed it, while the messages it sends suggest
// hanging a stop under that same bet. On three candles — buy at 100, a low of 97, a
// close of 110 — the same script reads +10% held to the end and -2% with a stop two
// percent away. Those are not two views of one strategy. One of them is a strategy
// nobody ran.
//
// Its zero value is a replay that simulates no exits at all, which is what every
// existing call is: not filling the distances in is ordinary, and a report card
// produced without them has every figure it had before this model existed. That is
// deliberately not a flag beside the two figures — a flag can disagree with them, and
// nothing could say which of the two to believe.
type BacktestExitLevelsDomain struct {
	// Zero means there is no such exit, for each of the two independently. Only one
	// of them being set is an ordinary thing to ask for.
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
}

// NewBacktestExitLevelsDomain reads the two distances this run was given.
//
// Both refusals come from the check a bot's position plan already uses, in the same
// words. The same 150 typed into either of the two forms has to be answered by the
// same sentence, and a second copy of those sentences would drift apart on the first
// day somebody improved one of them.
func NewBacktestExitLevelsDomain(
	stopLoss decimal.Decimal, takeProfit decimal.Decimal,
) (BacktestExitLevelsDomain, error) {
	if stopLossError := validatedDistance(stopLoss, "停損距離"); stopLossError != nil {
		return BacktestExitLevelsDomain{}, stopLossError
	}

	if takeProfitError := validatedDistance(takeProfit, "停利距離"); takeProfitError != nil {
		return BacktestExitLevelsDomain{}, takeProfitError
	}

	return BacktestExitLevelsDomain{stopLoss: stopLoss, takeProfit: takeProfit}, nil
}

// PricesFrom places this position's exits around one entry price.
//
// A stop is the price moving against the position and a target is it moving in favour,
// so a long's stop sits below its entry and its target above, and a short's are the
// mirror image. Getting that backwards is the one mistake here that cannot be seen: a
// stop on the wrong side is still a perfectly plausible price.
//
// **It takes the leverage because a loan being called in is a third exit on the side
// the stop is already on, and only the nearer of two same-side exits can ever happen.**
// Which is nearer is decided here, once, from the two distances — before either has
// been turned into a price, so no reasoning about which way the position faces is
// needed to compare them. The alternative, asking a candle about both and picking,
// spreads one settled fact across every bar of the walk and grows a branch each time
// somebody thinks of another way to be forced out.
//
// A tie goes to the stop. The two prices are the same, so the choice costs nothing on
// the report card, and a caller who asked for a stop should be told the stop is what
// took them out.
//
// This settles *where* the prices are. Whether a candle reached one of them is the
// position's question — see BacktestPositionDomain.ExitOn — because that needs the
// direction too, and the position is the only thing holding both.
func (backtestExitLevelsDomain BacktestExitLevelsDomain) PricesFrom(
	direction vo.PositionDirectionVo,
	entryPrice decimal.Decimal,
	leverage BacktestLeverageDomain,
) vo.ExitPricesVo {
	isShort := direction == vo.PositionDirectionShort

	exitPricesVo := vo.ExitPricesVo{
		HasTakeProfit: backtestExitLevelsDomain.takeProfit.IsPositive(),
	}

	if exitPricesVo.HasTakeProfit {
		exitPricesVo.TakeProfitPrice = movedBy(
			entryPrice, backtestExitLevelsDomain.takeProfit, !isShort)
	}

	adverseDistance := backtestExitLevelsDomain.stopLoss
	adverseReason := vo.TradeExitReasonStopLoss
	hasAdverse := adverseDistance.IsPositive()

	if liquidationDistance, canBeLiquidated := leverage.AdverseDistance(); canBeLiquidated &&
		(!hasAdverse || liquidationDistance.LessThan(adverseDistance)) {
		adverseDistance = liquidationDistance
		adverseReason = vo.TradeExitReasonLiquidation
		hasAdverse = true
	}

	if hasAdverse {
		exitPricesVo.HasAdverse = true
		exitPricesVo.AdverseReason = adverseReason
		exitPricesVo.AdversePrice = movedBy(entryPrice, adverseDistance, isShort)
	}

	return exitPricesVo
}
