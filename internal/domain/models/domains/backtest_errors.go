package domains

import (
	"errors"
	"fmt"
	"time"
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
	// BacktestTradingModeField is which set of rules the replay trades by.
	BacktestTradingModeField = "tradingMode"
	// BacktestExitLevelsField is the pair of exit distances, together.
	//
	// One name covers both because they are filled in as one group on every screen
	// that offers them, and the refusal's own sentence already says which of the two
	// it is about — the same judgement the time range makes, where one name covers
	// two moments and a coarseness. A second constant would only repeat a word the
	// sentence has already said, and would cost every caller a second mapping to
	// keep in step.
	BacktestExitLevelsField = "exitLevels"
	// BacktestTransactionCostsField is the pair of cost rates, together.
	//
	// One name for the same reason the exit distances have one: they are two boxes
	// filled in as a single group, and the refusal's own sentence already says which
	// of the two it is about.
	BacktestTransactionCostsField = "transactionCosts"
	// BacktestLeverageField is the multiplier and the maintenance margin rate,
	// together.
	//
	// One name for the same reason the exit distances and the cost rates have one:
	// they are two boxes filled in as a single group — the second one means nothing
	// without the first — and the refusal's own sentence already says which of the
	// two it is about.
	BacktestLeverageField = "leverage"
	// BacktestSignalSourcesField is the set of signal sources a trading strategy
	// replays through — what to go and change when they disagree about coarseness,
	// or when there are none at all.
	BacktestSignalSourcesField = "signalSources"
	// BacktestSlippageField is how far every fill of a contract replay lands on the
	// wrong side of its price.
	BacktestSlippageField = "slippage"
	// BacktestMaintenanceMarginRateField is a maintenance margin rate a contract
	// replay was handed, which only the symbol's ladder may say.
	BacktestMaintenanceMarginRateField = "maintenanceMarginRate"
	// BacktestFillTimingField is at what price a replay fills its signals.
	BacktestFillTimingField = "fillTiming"
	// BacktestValidationStartTimeField is where a replay is split for validation.
	BacktestValidationStartTimeField = "validationStartTime"
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

// ErrBacktestTimeAllowanceSpent marks a replay that did not finish within its whole-run
// allowance. Nothing was wrong with what was asked; it simply could not be answered in
// time, and half a replay is not handed over as though it were a shorter one.
var ErrBacktestTimeAllowanceSpent = errors.New("backtest time allowance spent")

// BacktestTimeAllowanceSpent is that refusal in words a person can act on.
func BacktestTimeAllowanceSpent(allowance time.Duration) error {
	return fmt.Errorf("%w: 重演在 %s 內沒跑完，已中止——請縮短期間，或改用粗一點的彙總刻度",
		ErrBacktestTimeAllowanceSpent, allowance)
}
