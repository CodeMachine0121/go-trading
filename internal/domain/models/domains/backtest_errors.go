package domains

import (
	"errors"
	"fmt"
)

// ErrBacktestValidation marks a replay the caller got wrong: the stretch of market,
// the starting capital, or how much each opening stakes.
//
// It is one sentinel rather than several because every one of them is answered the
// same way — the caller changes a value and asks again — and because the two failures
// that need to be told apart already have their own sentinels and are reused as they
// are: a knob whose name does not match, and a script that could not run.
var ErrBacktestValidation = errors.New("backtest validation failed")

// notEnoughKCandlesForBacktest is what every "this stretch cannot be replayed"
// refusal says, whatever made it so — a stretch holding one candle, a stretch holding
// none, and a stretch whose end comes before its start are the same problem to
// whoever asked, and giving them three different sentences would only suggest three
// different fixes.
func notEnoughKCandlesForBacktest(availableKCandleCount int) error {
	return fmt.Errorf(
		"%w: 這段期間湊不出足夠的 K 線，目前湊得出 %d 根，至少需要 %d 根",
		ErrBacktestValidation, availableKCandleCount, minimumBacktestKCandleCount)
}
