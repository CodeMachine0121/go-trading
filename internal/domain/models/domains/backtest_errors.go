package domains

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrBacktestValidation marks a replay request the caller must fix; knob mismatches and script failures keep their own sentinels.
var ErrBacktestValidation = errors.New("backtest validation failed")

// Field names a refusal can point at; callers translate them into their own UI labels.
const (
	// BacktestTimeRangeField covers both moments and the aggregation interval.
	BacktestTimeRangeField           = "timeRange"
	BacktestInitialCapitalField      = "initialCapital"
	BacktestPositionSizingValueField = "positionSizingValue"
	BacktestTradingModeField         = "tradingMode"
	// BacktestExitLevelsField covers both exit distances; the message says which.
	BacktestExitLevelsField = "exitLevels"
	// BacktestTransactionCostsField covers both cost rates; the message says which.
	BacktestTransactionCostsField = "transactionCosts"
	// BacktestLeverageField covers the multiplier and maintenance margin rate; the message says which.
	BacktestLeverageField      = "leverage"
	BacktestSignalSourcesField = "signalSources"
	// BacktestSlippageField is how far every contract fill lands on the wrong side of its price.
	BacktestSlippageField = "slippage"
	// BacktestMaintenanceMarginRateField marks a rate the caller supplied, which only the symbol's ladder may set.
	BacktestMaintenanceMarginRateField = "maintenanceMarginRate"
	BacktestFillTimingField            = "fillTiming"
	BacktestValidationStartTimeField   = "validationStartTime"
)

// BacktestFieldName extracts the field a refusal is about, so callers never match on message text.
func BacktestFieldName(err error) (string, bool) {
	var fieldError *backtestFieldError
	if !errors.As(err, &fieldError) {
		return "", false
	}

	return fieldError.field, true
}

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

// notEnoughKCandlesForBacktest is the single refusal for any stretch too short to replay.
func notEnoughKCandlesForBacktest(availableKCandleCount int) error {
	return BacktestValidationFailure(BacktestTimeRangeField, fmt.Sprintf(
		"這段期間湊不出足夠的 K 線，目前湊得出 %d 根，至少需要 %d 根",
		availableKCandleCount, minimumBacktestKCandleCount))
}

// ErrBacktestTimeAllowanceSpent marks a replay that exceeded its run-time allowance; partial results are never returned.
var ErrBacktestTimeAllowanceSpent = errors.New("backtest time allowance spent")

func BacktestTimeAllowanceSpent(allowance time.Duration) error {
	return fmt.Errorf("%w: 重演在 %s 秒內沒跑完，已中止——請縮短期間，或改用粗一點的彙總刻度",
		ErrBacktestTimeAllowanceSpent, strconv.FormatFloat(allowance.Seconds(), 'f', -1, 64))
}
