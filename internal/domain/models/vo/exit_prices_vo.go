package vo

import "github.com/shopspring/decimal"

// ExitPricesVo is where one position's exits sit, worked out once when it was entered
// and never again.
//
// It is a value rather than a model because whether a candle reached one of these
// prices depends on which way the position faces, and the direction belongs to the
// position — this is only the prices it settled on.
//
// **There are two prices here, not three, and that is the whole idea.** A stop and a
// liquidation both sit on the side the price moves against the position, so the nearer
// of the two is reached first — always, on every candle, whatever the market does.
// That is geometry, not a guess, and it is knowable the moment the position opens. So
// they are collapsed into one adverse price at entry, carrying which of the two it
// turned out to be. Whoever reads a candle asks one question instead of sequencing a
// ladder of them, and the next reason a position gets forced out becomes one more
// candidate where they are chosen rather than one more branch where they are checked.
//
// Each price carries its own "is there one at all" rather than being read as absent
// when zero. A stop a hundred percent away is priced at exactly zero, which is absurd
// but arithmetically fine; reading that as "no stop" would silently answer a different
// question from the one that was asked.
type ExitPricesVo struct {
	// AdversePrice is the nearest price at which this position is taken off it
	// against its will, and AdverseReason says which kind of exit that turned out to
	// be — a stop the caller asked for, or the loan being called in.
	AdversePrice    decimal.Decimal
	HasAdverse      bool
	AdverseReason   TradeExitReasonVo
	TakeProfitPrice decimal.Decimal
	HasTakeProfit   bool
}
