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

// Names of the inputs a refusal can point at. They are this system's own words for
// what was asked, not any particular screen's: whoever is asking decides what it drew
// that input as, and translates.
const (
	// BacktestTimeRangeField is the stretch to replay — the pair of moments, together
	// with how coarse the candles are, since changing either one fixes the same
	// refusals.
	BacktestTimeRangeField = "timeRange"
	// BacktestInitialCapitalField is what the account starts with.
	BacktestInitialCapitalField = "initialCapital"
	// BacktestPositionSizingValueField is the figure beside the sizing mode.
	BacktestPositionSizingValueField = "positionSizingValue"
)

// BacktestFieldName digs out which input a refusal is about, when it is about one.
//
// A caller telling these apart by reading the message would be matching on prose
// written for a person — it changes whenever the wording improves. The name travels as
// a value instead, exactly as the mismatched-knob failure carries its knob's name.
func BacktestFieldName(err error) (string, bool) {
	var fieldError *backtestFieldError
	if !errors.As(err, &fieldError) {
		return "", false
	}

	return fieldError.field, true
}

// BacktestValidationFailure builds a refusal that names the input at fault.
func BacktestValidationFailure(field string, reason string) error {
	return &backtestFieldError{field: field, reason: reason}
}

type backtestFieldError struct {
	field  string
	reason string
}

func (fieldError *backtestFieldError) Error() string {
	return fmt.Sprintf("%v: %s", ErrBacktestValidation, fieldError.reason)
}

func (fieldError *backtestFieldError) Unwrap() error {
	return ErrBacktestValidation
}

// notEnoughKCandlesForBacktest is what every "this stretch cannot be replayed"
// refusal says, whatever made it so — a stretch holding one candle, a stretch holding
// none, and a stretch whose end comes before its start are the same problem to
// whoever asked, and giving them three different sentences would only suggest three
// different fixes.
func notEnoughKCandlesForBacktest(availableKCandleCount int) error {
	return BacktestValidationFailure(BacktestTimeRangeField, fmt.Sprintf(
		"這段期間湊不出足夠的 K 線，目前湊得出 %d 根，至少需要 %d 根",
		availableKCandleCount, minimumBacktestKCandleCount))
}
