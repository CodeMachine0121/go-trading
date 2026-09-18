package vo

import "github.com/shopspring/decimal"

// ExitPricesVo is where one position's two exits sit, worked out once when it was
// entered and never again.
//
// It is a value rather than a model because whether a candle reached one of these
// prices depends on which way the position faces, and the direction belongs to the
// position — this is only the pair of prices it settled on.
//
// Each price carries its own "is there one at all" rather than being read as absent
// when zero. A stop a hundred percent away is priced at exactly zero, which is
// absurd but arithmetically fine; reading that as "no stop" would silently answer a
// different question from the one that was asked.
type ExitPricesVo struct {
	StopLossPrice   decimal.Decimal
	HasStopLoss     bool
	TakeProfitPrice decimal.Decimal
	HasTakeProfit   bool
}
