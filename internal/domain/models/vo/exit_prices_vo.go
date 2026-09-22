package vo

import "github.com/shopspring/decimal"

// ExitPricesVo is where one position's exits sit, worked out once when it was entered
// and never again.
//
// It is a value rather than a model because it is only the two prices a position
// settled on; whether a candle reached one of them is the position's question.
//
// Each price carries its own "is there one at all" rather than being read as absent
// when zero. A stop a hundred percent away is priced at exactly zero, which is absurd
// but arithmetically fine; reading that as "no stop" would silently answer a different
// question from the one that was asked.
type ExitPricesVo struct {
	// StopLossPrice is where the price moving against this position takes it off,
	// and it sits below the entry: a spot position is only ever long.
	StopLossPrice   decimal.Decimal
	HasStopLoss     bool
	TakeProfitPrice decimal.Decimal
	HasTakeProfit   bool
}
